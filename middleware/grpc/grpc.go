package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"

	ratelimit "github.com/vnmchuo/ratelimiter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// KeyFunc extracts a rate-limiting key from the context and RPC metadata.
type KeyFunc func(ctx context.Context, info *grpc.UnaryServerInfo) (string, error)

// WeightFunc determines the request weight for weighted rate limiting (AllowN).
type WeightFunc func(ctx context.Context, req any, info *grpc.UnaryServerInfo) int

// RateLimiter returns a gRPC UnaryServerInterceptor that enforces rate limiting using the given Limiter.
func RateLimiter(limiter ratelimit.Limiter, keyFunc KeyFunc) grpc.UnaryServerInterceptor {
	return RateLimiterN(limiter, keyFunc, nil)
}

// RateLimiterN returns a gRPC UnaryServerInterceptor with support for weighted requests.
func RateLimiterN(limiter ratelimit.Limiter, keyFunc KeyFunc, weightFunc WeightFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		key, err := keyFunc(ctx, info)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to extract rate limit key: %v", err)
		}

		weight := 1
		if weightFunc != nil {
			if w := weightFunc(ctx, req, info); w > 0 {
				weight = w
			}
		}

		var res *ratelimit.Result
		if weight > 1 {
			res, err = limiter.AllowN(ctx, key, weight)
		} else {
			res, err = limiter.Allow(ctx, key)
		}

		if err != nil {
			return nil, status.Errorf(codes.Internal, "rate limiter error: %v", err)
		}

		// Attach rate limit trailers for client inspection
		trailer := metadata.Pairs(
			"x-ratelimit-limit", strconv.Itoa(res.Limit),
			"x-ratelimit-remaining", strconv.FormatInt(res.Remaining, 10),
			"x-ratelimit-reset", res.ResetAfter.String(),
		)
		_ = grpc.SetTrailer(ctx, trailer)

		if !res.Allowed {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded for key %q, retry after %s", key, res.ResetAfter)
		}

		return handler(ctx, req)
	}
}

// KeyFromMetadata returns a KeyFunc that looks up the given metadata key (e.g. "x-tenant-id", "x-api-key").
// If the key is not present in the incoming metadata, it falls back to fallbackKey.
func KeyFromMetadata(metaKey string, fallbackKey string) KeyFunc {
	return func(ctx context.Context, info *grpc.UnaryServerInfo) (string, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			vals := md.Get(metaKey)
			if len(vals) > 0 && vals[0] != "" {
				return vals[0], nil
			}
		}
		if fallbackKey != "" {
			return fallbackKey, nil
		}
		return "", fmt.Errorf("metadata key %q not found in request", metaKey)
	}
}

// KeyFromMethod returns a KeyFunc that uses the full gRPC method path as the rate limit key.
func KeyFromMethod() KeyFunc {
	return func(ctx context.Context, info *grpc.UnaryServerInfo) (string, error) {
		return info.FullMethod, nil
	}
}

// KeyFromPeerIP returns a KeyFunc that uses the client's IP address.
func KeyFromPeerIP() KeyFunc {
	return func(ctx context.Context, info *grpc.UnaryServerInfo) (string, error) {
		p, ok := peer.FromContext(ctx)
		if !ok || p.Addr == nil {
			return "unknown-peer", nil
		}
		host, _, err := net.SplitHostPort(p.Addr.String())
		if err != nil {
			return p.Addr.String(), nil
		}
		return host, nil
	}
}

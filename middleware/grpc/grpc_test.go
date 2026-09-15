package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	ratelimit "github.com/vnmchuo/ratelimiter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newTestLimiter(t *testing.T, limit int, window time.Duration) (*miniredis.Miniredis, ratelimit.Limiter) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := ratelimit.NewRedisStore(
		rdb,
		ratelimit.WithLimit(limit),
		ratelimit.WithWindow(window),
	)
	return mr, store
}

func dummyHandler(ctx context.Context, req any) (any, error) {
	return "ok", nil
}

func TestRateLimiter_AllowsWithinQuota(t *testing.T) {
	_, limiter := newTestLimiter(t, 2, time.Minute)
	interceptor := RateLimiter(limiter, KeyFromMethod())

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/TestMethod"}
	ctx := context.Background()

	// 1st call -> allowed
	resp, err := interceptor(ctx, nil, info, dummyHandler)
	if err != nil || resp != "ok" {
		t.Fatalf("expected call 1 to succeed, got resp=%v err=%v", resp, err)
	}

	// 2nd call -> allowed
	resp, err = interceptor(ctx, nil, info, dummyHandler)
	if err != nil || resp != "ok" {
		t.Fatalf("expected call 2 to succeed, got resp=%v err=%v", resp, err)
	}

	// 3rd call -> rate limited
	resp, err = interceptor(ctx, nil, info, dummyHandler)
	if err == nil {
		t.Fatalf("expected call 3 to be rate limited, but succeeded")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted code, got %v", err)
	}
}

func TestRateLimiter_KeyFromMetadata(t *testing.T) {
	_, limiter := newTestLimiter(t, 1, time.Minute)
	interceptor := RateLimiter(limiter, KeyFromMetadata("x-tenant-id", "default-tenant"))

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/TestMethod"}

	// Tenant A call
	ctxA := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-tenant-id", "tenant-a"))
	resp, err := interceptor(ctxA, nil, info, dummyHandler)
	if err != nil || resp != "ok" {
		t.Fatalf("expected tenant-a first call to succeed, got %v", err)
	}

	// Tenant A second call -> blocked
	_, err = interceptor(ctxA, nil, info, dummyHandler)
	if err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected tenant-a second call to be blocked, got %v", err)
	}

	// Tenant B call -> should succeed because it has its own quota
	ctxB := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-tenant-id", "tenant-b"))
	resp, err = interceptor(ctxB, nil, info, dummyHandler)
	if err != nil || resp != "ok" {
		t.Fatalf("expected tenant-b call to succeed, got %v", err)
	}
}

func TestRateLimiterN_Weighted(t *testing.T) {
	_, limiter := newTestLimiter(t, 5, time.Minute)
	interceptor := RateLimiterN(limiter, KeyFromMethod(), func(ctx context.Context, req any, info *grpc.UnaryServerInfo) int {
		if w, ok := req.(int); ok {
			return w
		}
		return 1
	})

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/BatchOp"}
	ctx := context.Background()

	// Consume 4 tokens out of 5
	resp, err := interceptor(ctx, 4, info, dummyHandler)
	if err != nil || resp != "ok" {
		t.Fatalf("expected 4 tokens to succeed, got %v", err)
	}

	// Consume 2 tokens (would exceed limit of 5, currently 4 used)
	_, err = interceptor(ctx, 2, info, dummyHandler)
	if err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected 2 tokens to be exhausted, got %v", err)
	}

	// Consume 1 token (4 + 1 <= 5) -> should succeed
	resp, err = interceptor(ctx, 1, info, dummyHandler)
	if err != nil || resp != "ok" {
		t.Fatalf("expected 1 token to succeed, got %v", err)
	}
}

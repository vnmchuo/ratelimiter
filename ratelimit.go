package ratelimit

import (
	"context"
	"time"
)

// Result represents the outcome of a rate limit check.
type Result struct {
	Allowed    bool          // True if the request is permitted.
	Remaining  int           // Number of requests remaining in the current window.
	Limit      int           // The total configured limit for the window.
	ResetAfter time.Duration // Time remaining until the rate limit window resets.
}

// Limiter defines the contract for rate limiting implementations.
// It allows for different backend stores (Redis, In-Memory, etc.) to be used interchangeably.
type Limiter interface {
	// Allow checks if a request for the given key is permitted within the configured time window.
	// Returns a Result containing the current status or an error if the store is unreachable.
	Allow(ctx context.Context, key string) (*Result, error)
}

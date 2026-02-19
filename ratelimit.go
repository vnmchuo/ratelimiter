package ratelimit

import (
	"context"
	"time"
)

// Result represents the outcome of a rate limit check.
type Result struct {
	Allowed    bool          // True if the request is permitted.
	Remaining  int64         // Number of units remaining in the current window.
	Limit      int           // The total configured limit for the window.
	ResetAfter time.Duration // Time remaining until the rate limit window resets.
}

// Limiter defines the contract for rate limiting implementations.
// It allows for different backend stores (Redis, In-Memory, etc.) to be used interchangeably.
type Limiter interface {
	// Allow checks if a single request for the given key is permitted within the configured
	// time window. Equivalent to AllowN(ctx, key, 1).
	// Returns a Result containing the current status or an error if the store is unreachable.
	Allow(ctx context.Context, key string) (*Result, error)

	// AllowN checks if n units can be consumed for the given key within the configured
	// time window. Useful for weighted requests (e.g., bulk operations).
	// Returns a Result containing the current status or an error if the store is unreachable.
	AllowN(ctx context.Context, key string, n int) (*Result, error)

	// Status returns the current rate limit state for the given key without consuming
	// any units. This is a read-only "peek" operation useful for checking remaining
	// quota before sending large or expensive requests.
	Status(ctx context.Context, key string) (*Result, error)
}

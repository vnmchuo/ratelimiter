package ratelimit

import (
	"context"
	"time"
)

type Result struct {
	Allowed    bool
	Remaining  int
	Limit      int
	ResetAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string) (*Result, error)
}

package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local clear_before = now - window

-- 1. Remove expired data outside the current time window
redis.call('ZREMRANGEBYSCORE', key, 0, clear_before)

-- 2. Count the number of requests currently in the window
local current_count = redis.call('ZCARD', key)

-- 3. Check if the count is still below the limit
if current_count < limit then
    -- Add the current request to the Sorted Set
    redis.call('ZADD', key, now, now)
    redis.call('PEXPIRE', key, window)
    return {1, limit - current_count - 1}
else
    return {0, 0}
end
`

type RedisStore struct {
	client *redis.Client
	config Config
}

func NewRedisStore(client *redis.Client, opts ...Option) *RedisStore {
	// Default config
	cfg := Config{
		Limit:  100,
		Window: time.Minute,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(&cfg)
	}

	return &RedisStore{
		client: client,
		config: cfg,
	}
}

func (s *RedisStore) Allow(ctx context.Context, key string) (*Result, error) {
	now := time.Now().UnixMilli()
	windowMS := s.config.Window.Milliseconds()

	raw, err := s.client.Eval(ctx, slidingWindowScript, []string{key}, now, windowMS, s.config.Limit).Result()
	if err != nil {
		return nil, err
	}

	res := raw.([]interface{})
	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)

	return &Result{
		Allowed:    allowed,
		Remaining:  int(remaining),
		Limit:      s.config.Limit,
		ResetAfter: s.config.Window,
	}, nil
}

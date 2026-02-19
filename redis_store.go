package ratelimit

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

// slidingWindowScriptN implements the sliding window counter algorithm consuming N units.
// KEYS[1]: the rate limit key
// ARGV[1]: current timestamp in milliseconds
// ARGV[2]: window size in milliseconds
// ARGV[3]: max limit
// ARGV[4]: number of units to consume (n)
// Returns: {allowed (0|1), remaining}
const slidingWindowScriptN = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local n = tonumber(ARGV[4])
local clear_before = now - window

-- 1. Remove expired entries outside the current time window
redis.call('ZREMRANGEBYSCORE', key, 0, clear_before)

-- 2. Count the number of units currently in the window
local current_count = redis.call('ZCARD', key)

-- 3. Check if consuming n units would exceed the limit
if current_count + n <= limit then
    -- 4. Add n unique entries to the sorted set (member = "timestamp:index:rand")
    for i = 0, n - 1 do
        local member = now .. ':' .. i .. ':' .. ARGV[5 + i]
        redis.call('ZADD', key, now, member)
    end
    redis.call('PEXPIRE', key, window)
    return {1, limit - current_count - n}
else
    return {0, limit - current_count}
end
`

// slidingWindowStatusScript is a read-only peek that returns the current usage
// without consuming any units. It still removes expired entries for accuracy.
// KEYS[1]: the rate limit key
// ARGV[1]: current timestamp in milliseconds
// ARGV[2]: window size in milliseconds
// ARGV[3]: max limit
// Returns: {current_count, remaining}
const slidingWindowStatusScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local clear_before = now - window

-- Remove expired entries for an accurate count (does not modify the effective state)
redis.call('ZREMRANGEBYSCORE', key, 0, clear_before)

local current_count = redis.call('ZCARD', key)
local remaining = limit - current_count
if remaining < 0 then remaining = 0 end
return {current_count, remaining}
`

// RedisStore implements the Limiter interface using Redis as the backend.
// It utilizes Lua scripting to ensure atomic increments and window management.
type RedisStore struct {
	client *redis.Client
	config Config
}

// NewRedisStore initializes a new RedisStore with the provided Redis client and options.
// If no options are provided, it defaults to a limit of 100 requests per minute.
func NewRedisStore(client *redis.Client, opts ...Option) *RedisStore {
	cfg := Config{
		Limit:  100,
		Window: time.Minute,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &RedisStore{
		client: client,
		config: cfg,
	}
}

// Allow checks if a single request (1 unit) is permitted for the given key.
// It delegates to AllowN with n=1, maintaining full backward compatibility.
func (s *RedisStore) Allow(ctx context.Context, key string) (*Result, error) {
	return s.AllowN(ctx, key, 1)
}

// AllowN checks if n units can be consumed for the given key within the configured
// time window. The operation is atomic via Lua scripting and safe for distributed use.
func (s *RedisStore) AllowN(ctx context.Context, key string, n int) (*Result, error) {
	if n <= 0 {
		return nil, fmt.Errorf("ratelimit: n must be greater than 0, got %d", n)
	}

	now := time.Now().UnixMilli()
	windowMS := s.config.Window.Milliseconds()

	// Build ARGV: [now, windowMS, limit, n, rand1, rand2, ..., randN]
	args := make([]interface{}, 4+n)
	args[0] = now
	args[1] = windowMS
	args[2] = s.config.Limit
	args[3] = n
	for i := 0; i < n; i++ {
		args[4+i] = rand.Int63() //nolint:gosec // non-cryptographic uniqueness for member keys
	}

	raw, err := s.client.Eval(ctx, slidingWindowScriptN, []string{key}, args...).Result()
	if err != nil {
		return nil, err
	}

	res := raw.([]interface{})
	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)

	return &Result{
		Allowed:    allowed,
		Remaining:  remaining,
		Limit:      s.config.Limit,
		ResetAfter: s.config.Window,
	}, nil
}

// Status returns the current rate limit state for the given key without consuming
// any units. This is a lightweight "peek" useful for checking quota before expensive ops.
// The Allowed field is always true since no units are consumed; callers should check Remaining.
func (s *RedisStore) Status(ctx context.Context, key string) (*Result, error) {
	now := time.Now().UnixMilli()
	windowMS := s.config.Window.Milliseconds()

	raw, err := s.client.Eval(ctx, slidingWindowStatusScript, []string{key}, now, windowMS, s.config.Limit).Result()
	if err != nil {
		return nil, err
	}

	res := raw.([]interface{})
	remaining := res[1].(int64)

	return &Result{
		Allowed:    remaining > 0,
		Remaining:  remaining,
		Limit:      s.config.Limit,
		ResetAfter: s.config.Window,
	}, nil
}

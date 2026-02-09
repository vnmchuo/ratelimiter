package ratelimit

import "time"

// Config holds the parameters for the rate limiting window.
type Config struct {
	Limit  int           // Maximum number of allowed requests.
	Window time.Duration // The duration of the sliding window.
}

// Option is a functional configuration for the RedisStore.
type Option func(*Config)

// WithLimit sets the maximum number of requests allowed within the window.
func WithLimit(limit int) Option {
	return func(c *Config) {
		c.Limit = limit
	}
}

// WithWindow sets the duration for the rate limiting sliding window.
func WithWindow(window time.Duration) Option {
	return func(c *Config) {
		c.Window = window
	}
}

package ratelimit

import "time"

type Config struct {
	Limit  int
	Window time.Duration
}

type Option func(*Config)

func WithLimit(limit int) Option {
	return func(c *Config) {
		c.Limit = limit
	}
}

func WithWindow(window time.Duration) Option {
	return func(c *Config) {
		c.Window = window
	}
}


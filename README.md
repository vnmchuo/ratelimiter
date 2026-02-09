# Distributed Rate Limiter for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/virgiliusnanamanek02/ratelimiter.svg)](https://pkg.go.dev/github.com/virgiliusnanamanek02/ratelimiter)
[![Go Report Card](https://goreportcard.com/badge/github.com/virgiliusnanamanek02/ratelimiter)](https://goreportcard.com/report/github.com/virgiliusnanamanek02/ratelimiter)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A high-performance, distributed rate-limiting library for Go, powered by Redis and Lua scripting. This library implements the **Sliding Window Counter** algorithm to ensure precision and atomicity across multiple service instances.

## 🚀 Features

* **Distributed Architecture**: Synchronize rate limits across multiple nodes using Redis.
* **Sliding Window Algorithm**: Prevents traffic bursts at window boundaries, offering better precision than Fixed Window.
* **Atomic Operations**: Uses Redis Lua scripting to guarantee thread-safe operations without race conditions.
* **Framework Agnostic**: Core logic is decoupled from web frameworks.
* **Production Ready**: Built-in support for `context.Context` for timeout and cancellation handling.

## 🧠 Algorithm

This library uses the Sliding Window Counter algorithm implemented with Redis sorted sets and Lua scripting to ensure atomicity across distributed instances.


## 🛠 Installation

```bash
go get github.com/virgiliusnanamanek02/ratelimiter@v1.0.5
```

> 💡 **Note**: Make sure the version tag `v1.0.5` exists in the repository. Otherwise, use `@latest`.

## 💡 Quick Start

```go
package main

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/virgiliusnanamanek02/ratelimiter"
)

func main() {
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // Create a new limiter: 100 requests per minute
    limiter := ratelimiter.NewRedisStore(
        rdb,
        ratelimiter.WithLimit(100),
        ratelimiter.WithWindow(time.Minute),
    )

    res, err := limiter.Allow(context.Background(), "user-123")
    if err != nil {
        panic(err)
    }

    if res.Allowed {
        // Proceed with request
    } else {
        // Handle rate limit exceeded (e.g., HTTP 429)
    }
}
```

## 🔌 Framework Integration

This library is designed to be easily wrapped into any middleware.

### Gin Example

```go
import (
    ginmw "github.com/virgiliusnanamanek02/ratelimiter/middleware"
)

// ...
r := gin.Default()
r.Use(ginmw.RateLimiter(limiter, func(c *gin.Context) string {
    return c.ClientIP()
}))
```

## 📊 Benchmarks

Run the benchmarks on your machine to verify performance:

```bash
go test -bench=. -benchmem ./...
```

Benchmark results will vary depending on hardware, Redis configuration, and network latency.

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.

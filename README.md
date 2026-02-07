# Distributed Rate Limiter for Go

A high-performance, distributed rate-limiting library for Go, powered by Redis and Lua scripting. This library implements the **Sliding Window Counter** algorithm to ensure precision and atomicity across multiple service instances.

## 🚀 Features

* **Distributed Architecture**: Synchronize rate limits across multiple nodes using Redis.
* **Sliding Window Algorithm**: Prevents traffic bursts at window boundaries, offering better precision than Fixed Window.
* **Atomic Operations**: Uses Redis Lua scripting to guarantee thread-safe operations without race conditions.
* **Framework Agnostic**: Core logic is decoupled from web frameworks, with ready-to-use middleware for Gin and Echo.
* **Production Ready**: Built-in support for `context.Context` for timeout and cancellation handling.

## 🛠 Installation

```bash
go get github.com/virgiliusnanamanek02/rate-limiter-go

```

## 💡 Quick Start

```go
import (
    "context"
    "time"
    "github.com/redis/go-redis/v9"
    "github.com/virgiliusnanamanek02/rate-limiter-go"
)

func main() {
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    
    // Create a new limiter: 100 requests per minute
    limiter := ratelimit.NewRedisStore(
        rdb,
        ratelimit.WithLimit(100),
        ratelimit.WithWindow(time.Minute),
    )

    res, _ := limiter.Allow(context.Background(), "user-123")
    
    if res.Allowed {
        // Proceed with request
    } else {
        // Handle rate limit exceeded
    }
}

```

## 📊 Benchmarks

Run the benchmarks on your machine:

```bash
go test -bench=. -benchmem

```

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.

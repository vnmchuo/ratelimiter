# Distributed Rate Limiter for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/vnmchuo/ratelimiter.svg)](https://pkg.go.dev/github.com/vnmchuo/ratelimiter)
[![Go Report Card](https://goreportcard.com/badge/github.com/vnmchuo/ratelimiter)](https://goreportcard.com/report/github.com/vnmchuo/ratelimiter)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A high-performance, distributed rate-limiting library for Go, powered by Redis and Lua scripting. This library implements the **Sliding Window Counter** algorithm to ensure precision and atomicity across multiple service instances.

## 🚀 Features

* **Distributed Architecture**: Synchronize rate limits across multiple nodes using Redis.
* **Sliding Window Algorithm**: Prevents traffic bursts at window boundaries, offering better precision than Fixed Window.
* **Atomic Operations**: Uses Redis Lua scripting to guarantee thread-safe operations without race conditions.
* **Weighted Requests**: `AllowN` lets you consume multiple units in a single atomic call.
* **Read-only Peek**: `Status` lets you inspect remaining quota without consuming it.
* **Framework Agnostic**: Core logic is decoupled from web frameworks.
* **Production Ready**: Built-in support for `context.Context` for timeout and cancellation handling.

## 🛠 Installation

```bash
go get github.com/vnmchuo/ratelimiter@v1.2.0
```

## 💡 Quick Start

### Allow — single unit per call

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    ratelimiter "github.com/vnmchuo/ratelimiter"
)

func main() {
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // 100 requests per minute
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
        fmt.Printf("OK — %d remaining\n", res.Remaining)
    } else {
        fmt.Println("Rate limit exceeded (HTTP 429)")
    }
}
```

### AllowN — consume N units atomically

Use `AllowN` for weighted requests such as bulk API calls, large file uploads, or multi-item operations where a single request represents more than one logical unit of work.

```go
// Consume 10 units at once (e.g., a batch request that fetches 10 records)
res, err := limiter.AllowN(context.Background(), "user-123", 10)
if err != nil {
    panic(err)
}

if res.Allowed {
    fmt.Printf("Batch accepted — %d units remaining\n", res.Remaining)
} else {
    fmt.Printf("Not enough quota (only %d units left)\n", res.Remaining)
}
```

### Status — peek without consuming

Use `Status` to inspect remaining quota before committing to an expensive operation.

```go
s, err := limiter.Status(context.Background(), "user-123")
if err != nil {
    panic(err)
}

fmt.Printf("Current usage: %d/%d — %d remaining\n",
    int64(s.Limit)-s.Remaining, int64(s.Limit), s.Remaining)

if s.Remaining >= 50 {
    // Safe to proceed with a large batch
}
```

> **Note:** `Status` removes expired entries from the window for accuracy but never adds new ones. It is safe to call frequently with no side effects on quota.

## 🔌 Framework Integration

### Gin Middleware

```go
import (
    ginmw "github.com/vnmchuo/ratelimiter/middleware/gin"
)

r := gin.Default()
r.Use(ginmw.RateLimiter(limiter, func(c *gin.Context) string {
    return c.ClientIP() // key per IP
}))
```

The middleware automatically sets `X-RateLimit-Limit` and `X-RateLimit-Remaining` response headers and returns HTTP 429 when the limit is exceeded.

### gRPC Interceptor

```go
import (
    grpcmw "github.com/vnmchuo/ratelimiter/middleware/grpc"
)

// Standard UnaryServerInterceptor with tenant metadata extraction
s := grpc.NewServer(
    grpc.UnaryInterceptor(grpcmw.RateLimiter(
        limiter,
        grpcmw.KeyFromMetadata("x-tenant-id", "default-tenant"),
    )),
)
```

The interceptor automatically returns `codes.ResourceExhausted` when the limit is exceeded and attaches `x-ratelimit-limit`, `x-ratelimit-remaining`, and `x-ratelimit-reset` trailing metadata.

## 📊 Benchmarks

Benchmarks were run on an Intel Core i5-1145G7 @ 2.60GHz (Windows, amd64) using an in-process `miniredis` instance to eliminate network overhead:

```
goos: windows
goarch: amd64
cpu: 11th Gen Intel(R) Core(TM) i5-1145G7 @ 2.60GHz
```

| Benchmark | Iterations | ns/op | B/op | allocs/op |
|---|---:|---:|---:|---:|
| `BenchmarkRedisStore_Allow` | 10,000 | 973,820 | ~34 KB | 793 |
| `BenchmarkRedisStore_AllowN` (n=5) | 10,000 | ~1,100,000 | ~55 KB | 971 |
| `BenchmarkRedisStore_Status` | 30,987 | 188,717 | ~20 KB | — |

> **Note**: Production numbers with a networked Redis instance will be higher, dominated by round-trip latency (~1–5 ms). `Status` is faster than `Allow/AllowN` because it never writes to Redis.

To run benchmarks on your own hardware:

```bash
go test -bench=BenchmarkRedisStore -benchmem -benchtime=5s .
```

## 🧠 Algorithm Comparison

This library uses the **Sliding Window Counter** algorithm. Here's how it compares to common alternatives:

| Property | Fixed Window | Token Bucket | **Sliding Window Counter** ✅ |
|---|---|---|---|
| **Burst at boundary** | ❌ Yes — 2× burst possible | ✅ Controlled | ✅ None — truly smooth |
| **Memory per key** | ✅ O(1) | ✅ O(1) | ⚠️ O(limit) sorted set |
| **Precision** | ❌ Low | ✅ High | ✅ High |
| **Distributed safe** | ✅ With atomic INCR | ⚠️ Complex to distribute | ✅ Lua script atomicity |
| **Weighted requests** | ⚠️ Possible | ✅ Native | ✅ Native (AllowN) |
| **Implementation** | Simple | Moderate | Moderate |

**Why Sliding Window?**

* **Fixed Window** has a well-known double-burst vulnerability: a client can send `limit` requests just before a window resets and `limit` requests immediately after, effectively sending `2×limit` in a short span.
* **Token Bucket** handles bursts well but is harder to implement correctly in a distributed setting — you need per-node state or complex synchronization.
* **Sliding Window** via a Redis sorted set gives exact per-key tracking over a rolling time range, with atomicity guaranteed by Lua scripting. The O(limit) memory cost per key is acceptable for typical API rate limits (e.g., ≤10,000 req/min/user).

## ⚠️ Known Limitations

| Limitation | Details |
|---|---|
| **Redis dependency** | The library requires a running Redis instance. There is no in-process or local fallback. If Redis is unavailable, all `Allow`/`AllowN`/`Status` calls return an error — design your service to fail open or closed accordingly. |
| **No local fallback** | There is no in-memory fallback store. Clients operating without Redis connectivity cannot rate-limit locally. |
| **Memory grows with limit** | Each key uses a Redis sorted set with up to `limit` members. High limit values (e.g., 1M req/day) may increase Redis memory usage; consider using a separate, smaller limiter for high-frequency use cases. |
| **Clock skew** | In multi-node deployments, clock skew between application nodes can cause slight inaccuracies in the window boundary. Using a Redis server time (`TIME` command) would eliminate this but would require an extra round-trip. |
| **No retry-after header** | The `ResetAfter` field in `Result` is set to the full window duration. A precise "retry after N seconds" value would require tracking the oldest entry's timestamp. |
| **Single Redis instance** | Production deployments should consider Redis Sentinel or Redis Cluster for high availability. |

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.

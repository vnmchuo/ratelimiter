# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [1.2.0] - 2026-09-15

### Added

- `middleware/grpc`: First-class gRPC `UnaryServerInterceptor` (`RateLimiter` and `RateLimiterN`) for rate limiting incoming RPC calls.
- Key extraction functions: `KeyFromMetadata(headerKey, fallback)`, `KeyFromMethod()`, and `KeyFromPeerIP()`.
- Automatic injection of `x-ratelimit-limit`, `x-ratelimit-remaining`, and `x-ratelimit-reset` into gRPC trailing metadata.
- Returns standard `codes.ResourceExhausted` with informative details when rate limit is reached.
- Unit tests in `middleware/grpc/grpc_test.go` using miniredis.

### Added

- `AllowN(ctx context.Context, key string, n int) (*Result, error)` method to the `Limiter` interface and `RedisStore`. Atomically consumes N units in a single Lua script call, enabling weighted rate limiting for bulk or expensive operations.
- `Status(ctx context.Context, key string) (*Result, error)` method — a read-only "peek" that returns current quota without consuming any units. Useful for pre-flight checks before large requests.
- Input validation in `AllowN`: returns an error immediately for `n ≤ 0`.
- `slidingWindowScriptN` Lua script (replaces previous script internally) — parameterised `n`, inserts N unique sorted-set members atomically.
- `slidingWindowStatusScript` Lua script — read-only, cleans expired entries but never writes, giving an accurate snapshot.
- New tests: `TestRedisStore_AllowN_VariousN`, `TestRedisStore_AllowN_ExceedsLimit`, `TestRedisStore_AllowN_InvalidN`, `TestRedisStore_AllowN_Sequential`, `TestRedisStore_Status`, `TestRedisStore_Status_DoesNotConsume`, `TestRedisStore_ConcurrentAllowN`.
- New benchmarks: `BenchmarkRedisStore_AllowN`, `BenchmarkRedisStore_Status`.
- README: `AllowN` and `Status` usage examples, real benchmark table, Algorithm Comparison section (Sliding Window vs Fixed Window vs Token Bucket), Known Limitations section.

### Changed

- `Result.Remaining` type changed from `int` to `int64` to align with Redis int64 return values and eliminate internal casting.
- `Allow` now delegates to `AllowN(ctx, key, 1)` — behaviour is identical, fully backward compatible.

### Fixed

- Removed redundant `int()` cast on `remaining` in `redis_store.go`.

---

## [1.0.8] - 2026-02-19

### Changed

- Updated GitHub username in module path and examples from `virgiliusnanamanek02` to `vnmchuo`.
- Bumped version references to `v1.0.8`.

---

## [1.0.3] - 2026-02-16

### Added

- GoDoc comments on all exported types, functions, and methods.
- Integration guide and badges (Go Reference, Go Report Card, MIT License) in README.

### Changed

- Translated all inline code comments from Indonesian to English.
- Improved code formatting and style consistency.

---

## [1.0.2] - 2026-02-16

### Fixed

- Resolved CI pipeline failures (race detector, `go vet`, `gofmt` checks).
- Tidied `go.mod` / `go.sum`.

---

## [1.0.1] - 2026-02-16

### Fixed

- Minor CI configuration corrections.

---

## [1.0.0] - 2026-02-14

### Added

- Initial public release.
- `RedisStore` implementing the `Limiter` interface with a **Sliding Window Counter** algorithm backed by Redis sorted sets and Lua scripting.
- `Allow(ctx, key)` — single-unit rate limit check, atomic and distributed-safe.
- `Result` struct with `Allowed`, `Remaining`, `Limit`, and `ResetAfter` fields.
- Functional options: `WithLimit(n)` and `WithWindow(d)`.
- Gin middleware (`middleware/gin`) with `X-RateLimit-Limit` and `X-RateLimit-Remaining` headers and automatic HTTP 429 response.
- Gin usage example in `examples/gin/`.
- Unit tests using `miniredis` (in-process Redis mock) with window-sliding simulation.
- GitHub Actions CI: `gofmt`, `go vet`, race detector, coverage.
- MIT License.

---

[Unreleased]: https://github.com/vnmchuo/ratelimiter/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/vnmchuo/ratelimiter/compare/v1.0.8...v1.1.0
[1.0.8]: https://github.com/vnmchuo/ratelimiter/compare/v1.0.3...v1.0.8
[1.0.3]: https://github.com/vnmchuo/ratelimiter/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/vnmchuo/ratelimiter/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/vnmchuo/ratelimiter/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/vnmchuo/ratelimiter/releases/tag/v1.0.0

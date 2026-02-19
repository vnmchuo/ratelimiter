package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestStore is a helper that creates an in-memory Redis store for testing.
func newTestStore(t *testing.T, limit int, window time.Duration) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisStore(client, WithLimit(limit), WithWindow(window))
	return store, mr
}

// ----------------------------------------------------------------------------
// Allow tests
// ----------------------------------------------------------------------------

func TestRedisStore_Allow(t *testing.T) {
	store, mr := newTestStore(t, 2, time.Second)
	ctx := context.Background()
	key := "test-allow"

	// Request 1: should be allowed, 1 remaining
	res, err := store.Allow(ctx, key)
	if err != nil {
		t.Fatalf("request 1 error: %v", err)
	}
	if !res.Allowed || res.Remaining != 1 {
		t.Errorf("request 1: want allowed=true rem=1, got allowed=%v rem=%d", res.Allowed, res.Remaining)
	}

	// Request 2: should be allowed, 0 remaining
	res, err = store.Allow(ctx, key)
	if err != nil {
		t.Fatalf("request 2 error: %v", err)
	}
	if !res.Allowed || res.Remaining != 0 {
		t.Errorf("request 2: want allowed=true rem=0, got allowed=%v rem=%d", res.Allowed, res.Remaining)
	}

	// Request 3: should be denied (limit exceeded)
	res, err = store.Allow(ctx, key)
	if err != nil {
		t.Fatalf("request 3 error: %v", err)
	}
	if res.Allowed {
		t.Error("request 3: expected blocked, but was allowed")
	}

	// Advance time past window — request 4 should be allowed again
	mr.FastForward(time.Second + 100*time.Millisecond)

	res, err = store.Allow(ctx, key)
	if err != nil {
		t.Fatalf("request 4 error: %v", err)
	}
	if !res.Allowed {
		t.Error("request 4: should be allowed after window shift")
	}
}

// ----------------------------------------------------------------------------
// AllowN tests
// ----------------------------------------------------------------------------

func TestRedisStore_AllowN_VariousN(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		n             int
		wantAllowed   bool
		wantRemaining int64
	}{
		{"n=1 within limit", 10, 1, true, 9},
		{"n=5 within limit", 10, 5, true, 5},
		{"n=10 exact limit", 10, 10, true, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newTestStore(t, tc.limit, time.Minute)
			ctx := context.Background()

			res, err := store.AllowN(ctx, "key-"+tc.name, tc.n)
			if err != nil {
				t.Fatalf("AllowN error: %v", err)
			}
			if res.Allowed != tc.wantAllowed {
				t.Errorf("allowed: want %v, got %v", tc.wantAllowed, res.Allowed)
			}
			if res.Remaining != tc.wantRemaining {
				t.Errorf("remaining: want %d, got %d", tc.wantRemaining, res.Remaining)
			}
		})
	}
}

func TestRedisStore_AllowN_ExceedsLimit(t *testing.T) {
	// Limit = 3. AllowN(2) → OK (1 left). AllowN(2) → DENIED (only 1 left, need 2).
	store, _ := newTestStore(t, 3, time.Minute)
	ctx := context.Background()
	key := "test-allowN-exceed"

	res, err := store.AllowN(ctx, key, 2)
	if err != nil {
		t.Fatalf("first AllowN error: %v", err)
	}
	if !res.Allowed || res.Remaining != 1 {
		t.Errorf("first AllowN: want allowed=true rem=1, got allowed=%v rem=%d", res.Allowed, res.Remaining)
	}

	res, err = store.AllowN(ctx, key, 2)
	if err != nil {
		t.Fatalf("second AllowN error: %v", err)
	}
	if res.Allowed {
		t.Errorf("second AllowN: expected denied, but was allowed (remaining=%d)", res.Remaining)
	}
	if res.Remaining != 1 {
		t.Errorf("second AllowN remaining: want 1 (reflects unpacked count), got %d", res.Remaining)
	}
}

func TestRedisStore_AllowN_InvalidN(t *testing.T) {
	store, _ := newTestStore(t, 10, time.Minute)
	ctx := context.Background()

	_, err := store.AllowN(ctx, "key", 0)
	if err == nil {
		t.Error("AllowN(0): expected error, got nil")
	}

	_, err = store.AllowN(ctx, "key", -1)
	if err == nil {
		t.Error("AllowN(-1): expected error, got nil")
	}
}

func TestRedisStore_AllowN_Sequential(t *testing.T) {
	// Limit=5, send 3 requests of n=2 each. Only first two (n=2, n=2) fit (total=4 ≤ 5).
	// Third (n=2 would make total=6 > 5) is denied.
	store, _ := newTestStore(t, 5, time.Minute)
	ctx := context.Background()
	key := "test-allowN-seq"

	res1, err := store.AllowN(ctx, key, 2)
	if err != nil || !res1.Allowed {
		t.Fatalf("AllowN(2) #1: want allowed, got allowed=%v err=%v", res1.Allowed, err)
	}

	res2, err := store.AllowN(ctx, key, 2)
	if err != nil || !res2.Allowed {
		t.Fatalf("AllowN(2) #2: want allowed, got allowed=%v err=%v", res2.Allowed, err)
	}
	if res2.Remaining != 1 {
		t.Errorf("AllowN(2) #2 remaining: want 1, got %d", res2.Remaining)
	}

	res3, err := store.AllowN(ctx, key, 2)
	if err != nil {
		t.Fatalf("AllowN(2) #3 error: %v", err)
	}
	if res3.Allowed {
		t.Errorf("AllowN(2) #3: expected denied (only 1 left), got allowed")
	}
}

// ----------------------------------------------------------------------------
// Status tests
// ----------------------------------------------------------------------------

func TestRedisStore_Status(t *testing.T) {
	store, _ := newTestStore(t, 5, time.Minute)
	ctx := context.Background()
	key := "test-status"

	// Status on empty key: 5 remaining
	s, err := store.Status(ctx, key)
	if err != nil {
		t.Fatalf("Status (empty) error: %v", err)
	}
	if s.Remaining != 5 {
		t.Errorf("Status (empty): want remaining=5, got %d", s.Remaining)
	}
	if !s.Allowed {
		t.Error("Status (empty): Allowed should be true when quota available")
	}

	// Consume 3 units via Allow
	for i := 0; i < 3; i++ {
		if _, err := store.Allow(ctx, key); err != nil {
			t.Fatalf("Allow #%d error: %v", i+1, err)
		}
	}

	// Status should show 2 remaining, and not have consumed anything extra
	s, err = store.Status(ctx, key)
	if err != nil {
		t.Fatalf("Status (after 3 allows) error: %v", err)
	}
	if s.Remaining != 2 {
		t.Errorf("Status (after 3 allows): want remaining=2, got %d", s.Remaining)
	}

	// Calling Status twice should give the same remaining (non-consuming)
	s2, err := store.Status(ctx, key)
	if err != nil {
		t.Fatalf("Status (second call) error: %v", err)
	}
	if s.Remaining != s2.Remaining {
		t.Errorf("Status is not idempotent: first=%d, second=%d", s.Remaining, s2.Remaining)
	}

	// Exhaust the remaining quota
	for i := 0; i < 2; i++ {
		if _, err := store.Allow(ctx, key); err != nil {
			t.Fatalf("exhaust Allow #%d error: %v", i+1, err)
		}
	}

	// Status on exhausted key: 0 remaining, Allowed=false
	s, err = store.Status(ctx, key)
	if err != nil {
		t.Fatalf("Status (exhausted) error: %v", err)
	}
	if s.Remaining != 0 {
		t.Errorf("Status (exhausted): want remaining=0, got %d", s.Remaining)
	}
	if s.Allowed {
		t.Error("Status (exhausted): Allowed should be false when quota is 0")
	}
}

func TestRedisStore_Status_DoesNotConsume(t *testing.T) {
	// Verify that calling Status N times does not reduce the quota.
	store, _ := newTestStore(t, 3, time.Minute)
	ctx := context.Background()
	key := "test-status-no-consume"

	for i := 0; i < 10; i++ {
		s, err := store.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status call #%d error: %v", i+1, err)
		}
		if s.Remaining != 3 {
			t.Errorf("Status call #%d: quota was consumed; want remaining=3, got %d", i+1, s.Remaining)
		}
	}
}

// ----------------------------------------------------------------------------
// Concurrent AllowN test (Race condition)
// ----------------------------------------------------------------------------

func TestRedisStore_ConcurrentAllowN(t *testing.T) {
	const (
		limit      = 20
		goroutines = 30
		n          = 1
	)

	store, _ := newTestStore(t, limit, time.Minute)
	ctx := context.Background()
	key := "test-concurrent-allowN"

	var (
		wg      sync.WaitGroup
		allowed atomic.Int64
		denied  atomic.Int64
	)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			res, err := store.AllowN(ctx, key, n)
			if err != nil {
				return
			}
			if res.Allowed {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}
	wg.Wait()

	totalAllowed := allowed.Load()
	totalDenied := denied.Load()

	if totalAllowed != limit {
		t.Errorf("concurrent AllowN: expected exactly %d allowed, got %d (denied=%d)", limit, totalAllowed, totalDenied)
	}
	if totalAllowed+totalDenied != goroutines {
		t.Errorf("concurrent AllowN: allowed+denied (%d) != goroutines (%d)", totalAllowed+totalDenied, goroutines)
	}
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

func BenchmarkRedisStore_Allow(b *testing.B) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisStore(client, WithLimit(1_000_000), WithWindow(time.Minute))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = store.Allow(ctx, "bench-allow")
	}
}

func BenchmarkRedisStore_AllowN(b *testing.B) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisStore(client, WithLimit(1_000_000_000), WithWindow(time.Minute))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = store.AllowN(ctx, "bench-allowN", 5)
	}
}

func BenchmarkRedisStore_Status(b *testing.B) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisStore(client, WithLimit(100), WithWindow(time.Minute))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = store.Status(ctx, "bench-status")
	}
}

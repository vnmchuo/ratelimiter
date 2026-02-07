package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisStore_Allow(t *testing.T) {
	// 1. Setup mock redis
	mr, _ := miniredis.Run()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// 2. Inisialisasi Limiter: 2 request per detik
	limit := 2
	window := time.Second
	store := NewRedisStore(client, WithLimit(limit), WithWindow(window))
	ctx := context.Background()
	key := "test-user"

	// Request ke-1: Harus Allowed
	res, _ := store.Allow(ctx, key)
	if !res.Allowed || res.Remaining != 1 {
		t.Errorf("Request 1 failed: got allowed=%v, rem=%d", res.Allowed, res.Remaining)
	}

	// Request ke-2: Harus Allowed
	res, _ = store.Allow(ctx, key)
	if !res.Allowed || res.Remaining != 0 {
		t.Errorf("Request 2 failed: got allowed=%v, rem=%d", res.Allowed, res.Remaining)
	}

	// Request ke-3: Harus Denied (Limit terlampaui)
	res, _ = store.Allow(ctx, key)
	if res.Allowed {
		t.Error("Request 3 should have been blocked")
	}

	// 3. Simulasi waktu berlalu (Sliding Window)
	mr.FastForward(time.Second + 100*time.Millisecond)

	// Request ke-4: Harus Allowed lagi karena window sudah bergeser
	res, _ = store.Allow(ctx, key)
	if !res.Allowed {
		t.Error("Request 4 should be allowed after window shift")
	}
}

func BenchmarkRedisStore_Allow(b *testing.B) {
	mr, _ := miniredis.Run()
	// defer mr.Stop()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisStore(client)
	ctx := context.Background()

	b.ResetTimer() // Start counting time from here
	for i := 0; i < b.N; i++ {
		_, _ = store.Allow(ctx, "bench-key")
	}
}

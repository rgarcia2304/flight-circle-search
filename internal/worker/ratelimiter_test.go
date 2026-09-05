package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_SemaphoreCapsConcurrency(t *testing.T) {
	rl := NewRateLimiter(1000, 3) // very high RPS, cap concurrent = 3
	const N = 20

	var inFlight atomic.Int64
	var mu sync.Mutex
	var maxObserved int64

	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Wait(context.Background()); err != nil {
				t.Errorf("wait: %v", err)
				return
			}
			cur := inFlight.Add(1)
			defer inFlight.Add(-1)
			mu.Lock()
			if cur > maxObserved {
				maxObserved = cur
			}
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			rl.Release()
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if maxObserved > 3 {
		t.Errorf("max in-flight = %d, want <= 3", maxObserved)
	}
	if maxObserved < 2 {
		t.Errorf("max in-flight = %d, want at least 2 (to prove parallelism)", maxObserved)
	}
}

func TestRateLimiter_TokenBucketThrottles(t *testing.T) {
	rl := NewRateLimiter(10, 100) // 10 RPS, high concurrency
	const N = 30
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Wait(context.Background()); err != nil {
				t.Errorf("wait: %v", err)
				return
			}
			rl.Release()
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	// 10 RPS means 30 calls should take at least ~2s (first 10 instant via burst).
	if elapsed < 1500*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 1.5s for 30 calls at 10 RPS", elapsed)
	}
}

func TestRateLimiter_ContextCancel(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer rl.Release()

	// Second call would block on the semaphore — verify it respects ctx.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := rl.Wait(ctx); err == nil {
		t.Error("expected context deadline exceeded, got nil")
	}
}

func TestRateLimiter_InflightCounter(t *testing.T) {
	rl := NewRateLimiter(1000, 5)
	if rl.InFlight() != 0 {
		t.Errorf("initial in-flight = %d, want 0", rl.InFlight())
	}
	if rl.MaxConcurrent() != 5 {
		t.Errorf("max concurrent = %d, want 5", rl.MaxConcurrent())
	}
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rl.InFlight() != 1 {
		t.Errorf("after one wait, in-flight = %d, want 1", rl.InFlight())
	}
	rl.Release()
	if rl.InFlight() != 0 {
		t.Errorf("after release, in-flight = %d, want 0", rl.InFlight())
	}
}

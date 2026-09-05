package worker

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_SemaphoreCapsConcurrency(t *testing.T) {
	rl := NewRateLimiter(1000, 3) // very high RPS, cap concurrent = 3
	const N = 20

	var mu sync.Mutex
	cond := make(chan struct{})
	var completed int

	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Wait(context.Background()); err != nil {
				t.Errorf("wait: %v", err)
				return
			}
			// Hold the slot until all 20 goroutines have acquired it, so the
			// semaphore is provably saturated. Then release so the test can finish.
			mu.Lock()
			completed++
			mu.Unlock()
			<-cond
			rl.Release()
		}()
	}

	// Wait for the semaphore to saturate (3 slots taken, 17 goroutines blocked).
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		c := completed
		mu.Unlock()
		if c >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only %d goroutines acquired the semaphore, want at least 3", c)
		case <-time.After(5 * time.Millisecond):
		}
	}

	// Now assert the in-flight count from the limiter's own counter is exactly 3.
	if got := rl.InFlight(); got != 3 {
		t.Errorf("in-flight after saturation = %d, want 3", got)
	}

	// Release all held goroutines.
	close(cond)
	wg.Wait()

	if got := rl.InFlight(); got != 0 {
		t.Errorf("in-flight after wg.Wait = %d, want 0", got)
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

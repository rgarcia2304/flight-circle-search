package worker

import (
	"context"
	"sync/atomic"

	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

// RateLimiter combines a token bucket (RPS cap) with a weighted semaphore (concurrent in-flight cap).
// Both layers protect the upstream provider from overload.
type RateLimiter struct {
	rps     rate.Limiter
	sem     *semaphore.Weighted
	inflight atomic.Int64
	maxInflight int64
}

// NewRateLimiter creates a rate limiter with the given RPS and max concurrent in-flight calls.
func NewRateLimiter(rps float64, maxConcurrent int) *RateLimiter {
	burst := int(rps)
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{
		rps:         *rate.NewLimiter(rate.Limit(rps), burst),
		sem:         semaphore.NewWeighted(int64(maxConcurrent)),
		maxInflight: int64(maxConcurrent),
	}
}

// Wait blocks until a slot is available, or ctx is cancelled.
func (l *RateLimiter) Wait(ctx context.Context) error {
	// Token bucket first — smoother rate, blocks too-fast callers.
	if err := l.rps.Wait(ctx); err != nil {
		return err
	}
	// Then semaphore — hard cap on in-flight.
	if err := l.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	l.inflight.Add(1)
	return nil
}

// Release returns one slot to the semaphore.
func (l *RateLimiter) Release() {
	l.inflight.Add(-1)
	l.sem.Release(1)
}

// InFlight returns the current number of in-flight calls (for tests/observability).
func (l *RateLimiter) InFlight() int64 {
	return l.inflight.Load()
}

// MaxConcurrent returns the configured ceiling.
func (l *RateLimiter) MaxConcurrent() int64 {
	return l.maxInflight
}

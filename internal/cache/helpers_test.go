package cache

import (
	"testing"
	"time"
)

func newTestCache(t *testing.T) Cache {
	t.Helper()
	c, err := NewInMemoryCache()
	if err != nil {
		t.Fatalf("NewInMemoryCache: %v", err)
	}
	t.Cleanup(registerAdvancer(c))
	return c
}

func registerAdvancer(c *InMemoryCache) func() {
	prev := clockAdvancerFn
	clockAdvancerFn = func(d time.Duration) {
		c.advance(d)
	}
	return func() { clockAdvancerFn = prev }
}

var clockAdvancerFn func(d time.Duration)

func advanceClock(t *testing.T, d time.Duration) {
	t.Helper()
	if clockAdvancerFn == nil {
		t.Fatalf("advanceClock: no cache instance registered for clock advancement")
	}
	clockAdvancerFn(d)
}

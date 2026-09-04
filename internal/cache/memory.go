package cache

import (
	"context"
	"sync"
	"time"
)

type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
	now   time.Time
}

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

func NewInMemoryCache() (*InMemoryCache, error) {
	return &InMemoryCache{
		items: make(map[string]cacheEntry),
		now:   time.Now(),
	}, nil
}

func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	if entry.expiresAt.After(c.now) {
		return entry.value, nil
	}
	return nil, ErrCacheMiss
}

func (c *InMemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheEntry{
		value:     value,
		expiresAt: c.now.Add(ttl),
	}
	return nil
}

func (c *InMemoryCache) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

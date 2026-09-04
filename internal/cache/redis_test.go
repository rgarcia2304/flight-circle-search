package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisCache_Get_Set_RoundTrip(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	c, err := NewRedisCache(s.Addr())
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	if err := c.Set(ctx, "k1", []byte("hello"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("Get = %q, want %q", got, "hello")
	}
}

func TestRedisCache_Get_MissReturnsErrCacheMiss(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	c, err := NewRedisCache(s.Addr())
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	_, err = c.Get(context.Background(), "missing-key")
	if err == nil {
		t.Fatal("Get(missing) err = nil, want ErrCacheMiss")
	}
	if err != ErrCacheMiss {
		t.Errorf("Get(missing) err = %v, want ErrCacheMiss", err)
	}
}

func TestRedisCache_Set_RespectsTTL(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	c, err := NewRedisCache(s.Addr())
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	if err := c.Set(ctx, "k1", []byte("v"), 10*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := c.Get(ctx, "k1"); err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	s.FastForward(50 * time.Millisecond)

	_, err = c.Get(ctx, "k1")
	if err != ErrCacheMiss {
		t.Errorf("Get after expiry err = %v, want ErrCacheMiss", err)
	}
}

func TestRedisCache_Set_ZeroTTL_NotRetrievable(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	c, err := NewRedisCache(s.Addr())
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	if err := c.Set(ctx, "k1", []byte("v"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	_, err = c.Get(ctx, "k1")
	if err != ErrCacheMiss {
		t.Errorf("Get with zero TTL err = %v, want ErrCacheMiss", err)
	}
}

func TestRedisCache_Set_OverwritesExistingValue(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	c, err := NewRedisCache(s.Addr())
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	if err := c.Set(ctx, "k1", []byte("first"), time.Minute); err != nil {
		t.Fatalf("Set first: %v", err)
	}
	if err := c.Set(ctx, "k1", []byte("second"), time.Minute); err != nil {
		t.Fatalf("Set second: %v", err)
	}

	got, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("Get = %q, want %q", got, "second")
	}
}

func TestRedisCache_ImplementsCacheInterface(t *testing.T) {
	var _ Cache = (*RedisCache)(nil)
}

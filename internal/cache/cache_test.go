package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCache_Get_Set_RoundTrip(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	want := []byte("hello-world")
	if err := c.Set(ctx, "k1", want, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Get = %q, want %q", got, want)
	}
}

func TestCache_Get_MissReturnsErrCacheMiss(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_, err := c.Get(ctx, "missing-key")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get(missing) err = %v, want ErrCacheMiss", err)
	}
}

func TestCache_Set_OverwritesExistingValue(t *testing.T) {
	c := newTestCache(t)
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

func TestCache_Set_RespectsTTL(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k1", []byte("v"), 10*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if _, err := c.Get(ctx, "k1"); err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}

	advanceClock(t, 50*time.Millisecond)

	_, err := c.Get(ctx, "k1")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get after expiry err = %v, want ErrCacheMiss", err)
	}
}

func TestCache_Set_ZeroTTL_NotRetrievable(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k1", []byte("v"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	_, err := c.Get(ctx, "k1")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get with zero TTL err = %v, want ErrCacheMiss", err)
	}
}

func TestCache_Set_EmptyValue(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k1", []byte{}, time.Minute); err != nil {
		t.Fatalf("Set empty: %v", err)
	}

	got, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Get = %q, want empty", got)
	}
}

func TestCache_Set_NegativeTTL_NotRetrievable(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k1", []byte("v"), -1*time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}

	_, err := c.Get(ctx, "k1")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get with negative TTL err = %v, want ErrCacheMiss", err)
	}
}

func TestCache_Keys_AreIsolated(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k1", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("Set k1: %v", err)
	}
	if err := c.Set(ctx, "k2", []byte("v2"), time.Minute); err != nil {
		t.Fatalf("Set k2: %v", err)
	}

	got1, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get k1: %v", err)
	}
	got2, err := c.Get(ctx, "k2")
	if err != nil {
		t.Fatalf("Get k2: %v", err)
	}
	if string(got1) != "v1" {
		t.Errorf("k1 = %q, want v1", got1)
	}
	if string(got2) != "v2" {
		t.Errorf("k2 = %q, want v2", got2)
	}
}

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func BenchmarkInMemoryCache_Get(b *testing.B) {
	c, err := NewInMemoryCache()
	if err != nil {
		b.Fatalf("NewInMemoryCache: %v", err)
	}
	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("value"), time.Hour); err != nil {
		b.Fatalf("Set: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(ctx, "k")
	}
}

func BenchmarkInMemoryCache_Set(b *testing.B) {
	c, err := NewInMemoryCache()
	if err != nil {
		b.Fatalf("NewInMemoryCache: %v", err)
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = c.Set(ctx, "k", []byte("value"), time.Hour)
	}
}

func BenchmarkInMemoryCache_Get_Set(b *testing.B) {
	c, err := NewInMemoryCache()
	if err != nil {
		b.Fatalf("NewInMemoryCache: %v", err)
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = c.Set(ctx, "k", []byte("value"), time.Hour)
		_, _ = c.Get(ctx, "k")
	}
}

func BenchmarkRedisCache_Get(b *testing.B) {
	s, err := miniredis.Run()
	if err != nil {
		b.Fatalf("miniredis.Run: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	rc, err := NewRedisCache(s.Addr())
	if err != nil {
		b.Fatalf("NewRedisCache: %v", err)
	}
		b.Cleanup(func() { _ = rc.Close() })
	ctx := context.Background()
	if err := rc.Set(ctx, "k", []byte("value"), time.Hour); err != nil {
		b.Fatalf("Set: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = rc.Get(ctx, "k")
	}
}

func BenchmarkRedisCache_Set(b *testing.B) {
	s, err := miniredis.Run()
	if err != nil {
		b.Fatalf("miniredis.Run: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	rc, err := NewRedisCache(s.Addr())
	if err != nil {
		b.Fatalf("NewRedisCache: %v", err)
	}
		b.Cleanup(func() { _ = rc.Close() })
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.Set(ctx, "k", []byte("value"), time.Hour)
	}
}

func BenchmarkFareKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FareKey("NYC", "LON", "2026-10-15")
	}
}

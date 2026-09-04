package cache

import (
	"context"
	"errors"
	"testing"
)

func TestFareKey_Format(t *testing.T) {
	got := FareKey("NYC", "LON", "2026-10-15")
	want := "fare:NYC:LON:2026-10-15"
	if got != want {
		t.Errorf("FareKey = %q, want %q", got, want)
	}
}

func TestFareKey_CaseSensitivity(t *testing.T) {
	keys := []string{
		FareKey("nyc", "lon", "2026-10-15"),
		FareKey("NYC", "LON", "2026-10-15"),
		FareKey("Nyc", "Lon", "2026-10-15"),
	}
	unique := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		unique[k] = struct{}{}
	}
	if len(unique) != len(keys) {
		t.Errorf("FareKey is not case-sensitive: keys=%v (expected all distinct)", keys)
	}
}

func TestFareKey_DistinctRoutes(t *testing.T) {
	a := FareKey("NYC", "LON", "2026-10-15")
	b := FareKey("LON", "NYC", "2026-10-15")
	if a == b {
		t.Errorf("origin/destination reversed should produce different keys, got same: %q", a)
	}
}

func TestFareKey_EmptyInputs(t *testing.T) {
	got := FareKey("", "", "")
	want := "fare:::"
	if got != want {
		t.Errorf("FareKey empty = %q, want %q", got, want)
	}
}

func TestFareKey_DistinctDates(t *testing.T) {
	a := FareKey("NYC", "LON", "2026-10-15")
	b := FareKey("NYC", "LON", "2026-10-16")
	if a == b {
		t.Errorf("different dates should produce different keys, got same: %q", a)
	}
}

func TestFareKey_RoundTripThroughCache(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	key := FareKey("NYC", "LON", "2026-10-15")
	if err := c.Set(ctx, key, []byte("cached"), defaultTTL); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "cached" {
		t.Errorf("Get = %q, want %q", got, "cached")
	}

	missKey := FareKey("NYC", "LON", "2026-10-16")
	if _, err := c.Get(ctx, missKey); !errors.Is(err, ErrCacheMiss) {
		t.Errorf("missKey Get err = %v, want ErrCacheMiss", err)
	}
}

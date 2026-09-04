//go:build live
// +build live

package fareprovider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rgarcia2304/flight-circle-search/internal/cache"
)

func TestLiveSearch_NYCTOLON(t *testing.T) {
	token := os.Getenv("TRAVELPAYOUTS_TOKEN")
	if token == "" {
		t.Fatal("TRAVELPAYOUTS_TOKEN env var required")
	}

	client := NewTravelpayouts(token, "", nil)
	fares, err := client.Search(context.Background(), SearchRequest{
		Origin:      "NYC",
		Destination: "LON",
		Date:        "2026-10-15",
	})

	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(fares) == 0 {
		t.Fatal("expected at least one fare, got 0 (may need a date with recent search activity)")
	}

	f := fares[0]
	t.Logf("Cheapest fare: %s %s %s %s $%.2f %s departing %s",
		f.Origin, f.Destination, f.OriginAirport, f.DestinationAirport,
		float64(f.Price)/100, f.Currency, f.DepartureAt.Format("2006-01-02"))

	if f.Price <= 0 {
		t.Errorf("Price = %d, want > 0", f.Price)
	}
	if f.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", f.Currency)
	}
	if f.DepartureAt.IsZero() {
		t.Error("DepartureAt is zero")
	}
	if f.Link == "" {
		t.Error("Link is empty")
	}
	if f.OriginAirport == "" || f.DestinationAirport == "" {
		t.Error("Airport codes should be populated")
	}
}

const liveCacheTestOrigin = "NYC"
const liveCacheTestDestination = "LON"
const liveCacheTestDate = "2026-10-15"

func newLiveCachedClient(t *testing.T) (*Travelpayouts, cache.Cache) {
	t.Helper()
	token := os.Getenv("TRAVELPAYOUTS_TOKEN")
	if token == "" {
		t.Fatal("TRAVELPAYOUTS_TOKEN env var required")
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	rc, err := cache.NewRedisCache(redisAddr)
	if err != nil {
		t.Fatalf("cache.NewRedisCache(%q): %v (start redis via 'docker compose up -d redis')", redisAddr, err)
	}
	t.Cleanup(func() { _ = rc.Close() })

	tp := NewTravelpayouts(token, "", nil)
	tp.SetCache(rc, time.Hour)
	return tp, rc
}

func clearRedisKey(t *testing.T, rc cache.Cache, key string) {
	t.Helper()
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer func() { _ = client.Close() }()
	if err := client.Del(context.Background(), key).Err(); err != nil {
		t.Fatalf("redis DEL %q: %v", key, err)
	}
}

func liveTestRequest() SearchRequest {
	return SearchRequest{
		Origin:      liveCacheTestOrigin,
		Destination: liveCacheTestDestination,
		Date:        liveCacheTestDate,
	}
}

func TestLiveSearch_CacheHit_ReturnsSameResult(t *testing.T) {
	tp, _ := newLiveCachedClient(t)
	req := liveTestRequest()
	ctx := context.Background()

	tpNoCache := NewTravelpayouts(os.Getenv("TRAVELPAYOUTS_TOKEN"), "", nil)
	first, err := tpNoCache.Search(ctx, req)
	if err != nil {
		t.Fatalf("first (uncached) Search: %v", err)
	}
	if len(first) == 0 {
		t.Skip("no fares returned for this date — cannot validate cache")
	}

	second, err := tp.Search(ctx, req)
	if err != nil {
		t.Fatalf("second (cached) Search: %v", err)
	}

	if len(first) != len(second) {
		t.Errorf("fare count: miss=%d hit=%d (expected equal)", len(first), len(second))
	}
	if len(first) == 0 || len(second) == 0 {
		t.Logf("miss=%d fares, hit=%d fares", len(first), len(second))
		return
	}
	if first[0].Price != second[0].Price {
		t.Errorf("cheapest fare price: miss=%d hit=%d", first[0].Price, second[0].Price)
	}
	if first[0].FlightNumber != second[0].FlightNumber {
		t.Errorf("cheapest fare flight: miss=%s hit=%s", first[0].FlightNumber, second[0].FlightNumber)
	}
}

func TestLiveSearch_CacheMissVsHit_Timing(t *testing.T) {
	tp, rc := newLiveCachedClient(t)
	req := liveTestRequest()
	ctx := context.Background()
	key := cache.FareKey(req.Origin, req.Destination, req.Date)
	clearRedisKey(t, rc, key)

	missStart := time.Now()
	missFares, err := tp.Search(ctx, req)
	if err != nil {
		t.Fatalf("miss Search: %v", err)
	}
	missLatency := time.Since(missStart)

	hitStart := time.Now()
	hitFares, err := tp.Search(ctx, req)
	if err != nil {
		t.Fatalf("hit Search: %v", err)
	}
	hitLatency := time.Since(hitStart)

	t.Logf("miss=%v hit=%v ratio=%.1fx", missLatency, hitLatency, float64(missLatency)/float64(hitLatency))

	if len(missFares) == 0 || len(hitFares) == 0 {
		t.Skip("no fares returned — cannot compare timing meaningfully")
	}

	if hitLatency*2 >= missLatency {
		t.Errorf("hit (%v) not meaningfully faster than miss (%v) — ratio < 2x", hitLatency, missLatency)
	}
}

func TestLiveSearch_CacheTTL_IsSet(t *testing.T) {
	_, rc := newLiveCachedClient(t)
	req := liveTestRequest()
	ctx := context.Background()
	key := cache.FareKey(req.Origin, req.Destination, req.Date)
	clearRedisKey(t, rc, key)

	tp := NewTravelpayouts(os.Getenv("TRAVELPAYOUTS_TOKEN"), "", nil)
	tp.SetCache(rc, time.Hour)
	if _, err := tp.Search(ctx, req); err != nil {
		t.Fatalf("Search: %v", err)
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer func() { _ = client.Close() }()

	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("redis TTL %q: %v", key, err)
	}
	t.Logf("TTL on %q = %v", key, ttl)

	if ttl <= 0 {
		t.Errorf("TTL = %v, want > 0 (key should have an expiry)", ttl)
	}
	if ttl < 55*time.Minute {
		t.Errorf("TTL = %v, want >= 55m (cache TTL should be ~1h)", ttl)
	}
	if ttl > 70*time.Minute {
		t.Errorf("TTL = %v, want <= 70m (cache TTL should be ~1h)", ttl)
	}

	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("redis EXISTS %q: %v", key, err)
	}
	if exists != 1 {
		t.Errorf("key %q exists = %d, want 1", key, exists)
	}
}

var _ = fmt.Sprintf

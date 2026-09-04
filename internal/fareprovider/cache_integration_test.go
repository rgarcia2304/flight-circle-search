package fareprovider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/rgarcia2304/flight-circle-search/internal/cache"
)

const fareCacheTTL = time.Hour

func newTestCache(t *testing.T) cache.Cache {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	rc, err := cache.NewRedisCache(s.Addr())
	if err != nil {
		t.Fatalf("cache.NewRedisCache: %v", err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	return rc
}

func newCachedClient(t *testing.T, baseURL string, c cache.Cache) *Travelpayouts {
	t.Helper()
	tp := NewTravelpayouts(testToken, baseURL, &http.Client{Timeout: 5 * time.Second})
	if c == nil {
		c = newTestCache(t)
	}
	tp.SetCache(c, fareCacheTTL)
	return tp
}

// ---------------------------------------------------------------------------
// Cache-aside behavior
// ---------------------------------------------------------------------------

func TestSearch_CacheHit_AvoidsNetworkCall(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	t.Cleanup(server.Close)

	client := newCachedClient(t, server.URL, nil)
	req := validRequest()

	first, err := client.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("first Search: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("first Search returned no fares")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("after 1st Search: server hits = %d, want 1", got)
	}

	second, err := client.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("second Search: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("after 2nd Search: server hits = %d, want 1 (cache hit should not re-hit network)", got)
	}
	if len(second) != len(first) {
		t.Errorf("2nd Search returned %d fares, want %d", len(second), len(first))
	}
}

func TestSearch_CacheMiss_PopulatesCache(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	t.Cleanup(server.Close)

	c := newTestCache(t)
	client := newCachedClient(t, server.URL, c)
	req := validRequest()

	if _, err := client.Search(context.Background(), req); err != nil {
		t.Fatalf("Search: %v", err)
	}

	key := cache.FareKey(req.Origin, req.Destination, req.Date)
	val, err := c.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("cache.Get(%q) after miss: %v (expected cache to be populated)", key, err)
	}
	if len(val) == 0 {
		t.Errorf("cache value for %q is empty", key)
	}
}

func TestSearch_CacheHit_FasterThanMiss(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	t.Cleanup(server.Close)

	c := newTestCache(t)
	client := newCachedClient(t, server.URL, c)
	req := validRequest()

	start := time.Now()
	if _, err := client.Search(context.Background(), req); err != nil {
		t.Fatalf("miss: %v", err)
	}
	missDuration := time.Since(start)

	start = time.Now()
	if _, err := client.Search(context.Background(), req); err != nil {
		t.Fatalf("hit: %v", err)
	}
	hitDuration := time.Since(start)

	if hitDuration >= missDuration {
		t.Errorf("hit (%v) not faster than miss (%v) — cache not providing performance benefit", hitDuration, missDuration)
	}
}

func TestSearch_DistinctRequests_HaveDistinctCacheKeys(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	t.Cleanup(server.Close)

	c := newTestCache(t)
	client := newCachedClient(t, server.URL, c)

	r1 := SearchRequest{Origin: "NYC", Destination: "LON", Date: "2026-10-15"}
	r2 := SearchRequest{Origin: "NYC", Destination: "LON", Date: "2026-10-16"}
	r3 := SearchRequest{Origin: "LON", Destination: "NYC", Date: "2026-10-15"}

	for _, req := range []SearchRequest{r1, r2, r3} {
		if _, err := client.Search(context.Background(), req); err != nil {
			t.Fatalf("Search(%+v): %v", req, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("server hits = %d, want 3 (3 distinct requests should each hit the network)", got)
	}
}

func TestSearch_CacheError_FallsBackToNetwork(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	t.Cleanup(server.Close)

	brokenCache := &brokenCache{}
	client := newCachedClient(t, server.URL, brokenCache)
	req := validRequest()

	fares, err := client.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("Search: %v (expected to fall back to network on cache error)", err)
	}
	if len(fares) == 0 {
		t.Error("Search returned no fares, expected network result")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server hits = %d, want 1 (cache error should not block network call)", got)
	}
}

func TestSearch_InvalidInput_DoesNotPopulateCache(t *testing.T) {
	c := newTestCache(t)
	tp := NewTravelpayouts(testToken, "http://unused.invalid", &http.Client{Timeout: time.Second})
	tp.SetCache(c, fareCacheTTL)

	if _, err := tp.Search(context.Background(), SearchRequest{Origin: "", Destination: "LON", Date: "2026-10-15"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	for _, k := range []string{
		cache.FareKey("NYC", "LON", "2026-10-15"),
		cache.FareKey("", "LON", "2026-10-15"),
		cache.FareKey("NYC", "", "2026-10-15"),
	} {
		if _, err := c.Get(context.Background(), k); !errors.Is(err, cache.ErrCacheMiss) {
			t.Errorf("cache should not contain %q after invalid input, got err=%v", k, err)
		}
	}
}

func TestSearch_NoCache_WorksWithoutCacheLayer(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	t.Cleanup(server.Close)

	client := NewTravelpayouts(testToken, server.URL, &http.Client{Timeout: 5 * time.Second})
	req := validRequest()

	if _, err := client.Search(context.Background(), req); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := client.Search(context.Background(), req); err != nil {
		t.Fatalf("second: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("without cache: server hits = %d, want 2 (no cache = always network)", got)
	}
}

type brokenCache struct{}

func (b *brokenCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, errors.New("simulated cache failure")
}

func (b *brokenCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return errors.New("simulated cache failure")
}

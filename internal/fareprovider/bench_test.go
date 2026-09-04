package fareprovider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/rgarcia2304/flight-circle-search/internal/cache"
)

func benchmarkSearch(b *testing.B, useCache bool, cached bool) {
	fixtureBytes, err := fixtureBytes("testdata/travelpayouts_success.json")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	fixture := string(fixtureBytes)

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	var tp *Travelpayouts
	if useCache {
		s, err := miniredis.Run()
		if err != nil {
			b.Fatalf("miniredis.Run: %v", err)
		}
		b.Cleanup(func() { s.Close() })
		rc, err := cache.NewRedisCache(s.Addr())
		if err != nil {
			b.Fatalf("NewRedisCache: %v", err)
		}
		b.Cleanup(func() { _ = rc.Close() })
		tp = NewTravelpayouts(testToken, server.URL, &http.Client{})
		tp.SetCache(rc, time.Hour)
		if cached {
			_, _ = tp.Search(context.Background(), validRequest())
		}
	} else {
		tp = NewTravelpayouts(testToken, server.URL, &http.Client{})
		if cached {
			_, _ = tp.Search(context.Background(), validRequest())
		}
	}

	req := validRequest()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = tp.Search(context.Background(), req)
	}
}

func BenchmarkSearch_NoCache(b *testing.B) {
	benchmarkSearch(b, false, false)
}

func BenchmarkSearch_WithCache_Miss(b *testing.B) {
	benchmarkSearch(b, true, false)
}

func BenchmarkSearch_WithCache_Hit(b *testing.B) {
	benchmarkSearch(b, true, true)
}

func BenchmarkSearch_NoCache_ThenHit(b *testing.B) {
	fixtureBytes, err := fixtureBytes("testdata/travelpayouts_success.json")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	fixture := string(fixtureBytes)

	var serverHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	s, err := miniredis.Run()
	if err != nil {
		b.Fatalf("miniredis.Run: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	rc, err := cache.NewRedisCache(s.Addr())
	if err != nil {
		b.Fatalf("NewRedisCache: %v", err)
	}
	b.Cleanup(func() { _ = rc.Close() })

	tp := NewTravelpayouts(testToken, server.URL, &http.Client{})
	tp.SetCache(rc, time.Hour)
	req := validRequest()

	var missStart time.Time
	phase := 0

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if phase == 0 {
			missStart = time.Now()
			_, _ = tp.Search(context.Background(), req)
			phase = 1
		} else {
			_, _ = tp.Search(context.Background(), req)
		}
	}

	missLatency := time.Since(missStart)
	b.ReportMetric(missLatency.Seconds(), "miss_latency_s")
	b.ReportMetric(float64(serverHits.Load()), "api_calls")

	cachedHits := b.N - int(serverHits.Load())
	b.ReportMetric(float64(cachedHits), "cache_hits")
}

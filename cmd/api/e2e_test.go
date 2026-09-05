package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rgarcia2304/flight-circle-search/internal/db"
	"github.com/rgarcia2304/flight-circle-search/internal/fareprovider"
	"github.com/rgarcia2304/flight-circle-search/internal/jobengine"
	"github.com/rgarcia2304/flight-circle-search/internal/worker"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func TestLive_E2E(t *testing.T) {
	// Requires TRAVELPAYOUTS_API_KEY and docker-compose up -d postgres
	token := os.Getenv("TRAVELPAYOUTS_API_KEY")
	if token == "" {
		t.Skip("TRAVELPAYOUTS_API_KEY not set, skipping live E2E test")
	}

	// Requires docker-compose up -d postgres first.
	pool, err := pgxpool.New(context.Background(),
		"postgres://dev:dev@localhost:5432/flightsearch_test?sslmode=disable")
	if err != nil {
		t.Skipf("Cannot connect to postgres (is docker-compose up -d postgres run?): %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := db.RunRiverMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Clean test DB before/after.
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE search_jobs, search_job_results RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE search_jobs, search_job_results RESTART IDENTITY")
	}()

	repo := jobengine.NewPostgresRepository(pool)
	fp := fareprovider.NewTravelpayouts(token, "", nil)

	enqueuer := &riverEnqueuer{client: mustNewRiverClient(ctx, pool)}
	svc := jobengine.NewService(repo, newCSVAirportSource(), enqueuer)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", makeSubmitHandler(svc))
	mux.HandleFunc("GET /jobs/{id}", makeGetHandler(repo))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	fareWorker := worker.NewFareWorker(repo, fp, worker.NewRateLimiter(20, 20))
	workerClient := mustNewWorkerClient(ctx, pool, fareWorker)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := workerClient.Start(ctx); err != nil {
			t.Errorf("worker start: %v", err)
			return
		}
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = workerClient.Stop(shutdownCtx)
	}()

	time.Sleep(500 * time.Millisecond)

	jobID, totalPairs, err := postSubmit(ctx, server.URL+"/jobs", liveSubmitRequest{
		OriginLat:  40.6398, OriginLng: -73.7789, OriginR: 200,
		DestLat:    52.5597, DestLng:   13.2877,  DestR:    500,
		DepartFrom: "2026-10-15", DepartTo: "2026-10-15",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	t.Logf("Job submitted: id=%s pairs=%d", jobID, totalPairs)

	if totalPairs == 0 {
		t.Fatal("expected at least 1 pair, got 0")
	}

	job := pollUntilDone(ctx, t, server.URL+"/jobs/"+jobID+"?include=results")
	t.Logf("Job %s: status=%s completed=%d failed=%d/%d",
		jobID, job.Status, job.CompletedPairs, job.FailedPairs, job.TotalPairs)

	if job.TotalPairs != totalPairs {
		t.Errorf("total_pairs mismatch: got %d, want %d", job.TotalPairs, totalPairs)
	}

	hasResult := false
	for _, r := range job.Results {
		switch r.Status {
		case "complete":
			if len(r.Fare) == 0 {
				t.Logf("  ✓ %s→%s %s: complete, no fares available for this date", r.Origin, r.Destination, r.DepartureDate)
			} else {
				t.Logf("  ✓ %s→%s %s: complete with %d bytes fare data", r.Origin, r.Destination, r.DepartureDate, len(r.Fare))
				logFareDetails(t, r.Origin, r.Destination, r.Fare)
			}
			hasResult = true
		case "dead_letter":
			t.Logf("  ✗ %s→%s %s: dead-lettered (%s)", r.Origin, r.Destination, r.DepartureDate, deref(r.Error))
		case "failed":
			t.Logf("  ! %s→%s %s: failed (%s)", r.Origin, r.Destination, r.DepartureDate, deref(r.Error))
		}
	}
	if !hasResult && job.Status == "complete" {
		t.Log("No successful completions, but job reached terminal status (free tier may be throttled or no data).")
	}
}

func logFareDetails(t *testing.T, origin, dest string, raw json.RawMessage) {
	var fares []struct {
		Airline      string `json:"Airline"`
		FlightNumber string `json:"FlightNumber"`
		DepartureAt  string `json:"DepartureAt"`
		Origin       string `json:"Origin"`
		Destination  string `json:"Destination"`
		Price        int    `json:"Price"`
		Currency     string `json:"Currency"`
		Duration     int64  `json:"Duration"`
		Transfers    int    `json:"Transfers"`
		Link         string `json:"Link"`
	}
	if err := json.Unmarshal(raw, &fares); err != nil {
		t.Logf("      (could not parse fare payload: %v)", err)
		return
	}
	best := fares[0]
	price := float64(best.Price) / 100
	dur := time.Duration(best.Duration) * time.Minute
	transfers := "direct"
	if best.Transfers > 0 {
		transfers = fmt.Sprintf("%d stop(s)", best.Transfers)
	}
	depTime, _ := time.Parse(time.RFC3339, best.DepartureAt)
	depStr := depTime.Format("Mon Jan 2 15:04")
	t.Logf("      best: %s %s | %s | %s→%s | %s | $%.2f %s | %s",
		best.Airline, best.FlightNumber,
		depStr,
		best.Origin, best.Destination,
		transfers,
		price, best.Currency,
		dur.Round(time.Minute).String(),
	)
	t.Logf("      link: %s", best.Link)
}

func mustNewRiverClient(ctx context.Context, pool *pgxpool.Pool) *river.Client[pgx.Tx] {
	// Insert-only client: registers a worker for a queue the real worker doesn't use,
	// and the real worker uses "search". This client only inserts, never fetches.
	workers := river.NewWorkers()
	river.AddWorker(workers, &idleWorker{})

	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{"submit_only": {MaxWorkers: 1}},
		Workers: workers,
		Schema:  "public",
	})
	if err != nil {
		log.Fatalf("create river client: %v", err)
	}
	// Intentionally NOT calling c.Start() — this client only inserts, never fetches.
	return c
}

type idleWorker struct {
	river.WorkerDefaults[worker.SearchJobArgs]
}

func (w *idleWorker) Work(_ context.Context, _ *river.Job[worker.SearchJobArgs]) error { return nil }

func mustNewWorkerClient(ctx context.Context, pool *pgxpool.Pool, w *worker.FareWorker) *river.Client[pgx.Tx] {
	workers := river.NewWorkers()
	river.AddWorker(workers, w)

	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{"search": {MaxWorkers: 2}},
		Workers: workers,
	})
	if err != nil {
		log.Fatalf("create worker client: %v", err)
	}
	return c
}

type liveSubmitRequest struct {
	OriginLat  float64 `json:"origin_lat"`
	OriginLng  float64 `json:"origin_lng"`
	OriginR    float64 `json:"origin_r"`
	DestLat    float64 `json:"dest_lat"`
	DestLng    float64 `json:"dest_lng"`
	DestR      float64 `json:"dest_r"`
	DepartFrom string  `json:"depart_from"`
	DepartTo   string  `json:"depart_to"`
}

type liveJobStatus struct {
	ID             string       `json:"id"`
	Status         string       `json:"status"`
	TotalPairs     int          `json:"total_pairs"`
	CompletedPairs int          `json:"completed_pairs"`
	FailedPairs    int          `json:"failed_pairs"`
	ErrorSummary   *string      `json:"error_summary,omitempty"`
	Results        []liveResult `json:"results,omitempty"`
}

type liveResult struct {
	Origin        string          `json:"origin"`
	Destination   string          `json:"destination"`
	DepartureDate string          `json:"departure_date"`
	Status        string          `json:"status"`
	Fare          json.RawMessage `json:"fare,omitempty"`
	Error         *string         `json:"error,omitempty"`
}

func postSubmit(ctx context.Context, url string, req liveSubmitRequest) (string, int, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return "", 0, fmt.Errorf("submit failed (%d): %s", resp.StatusCode, data)
	}
	var r struct {
		JobID      string `json:"job_id"`
		TotalPairs int    `json:"total_pairs"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", 0, fmt.Errorf("decode: %w", err)
	}
	return r.JobID, r.TotalPairs, nil
}

func pollUntilDone(ctx context.Context, t *testing.T, url string) *liveJobStatus {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled")
		case <-timeout:
			var job liveJobStatus
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			resp, _ := http.DefaultClient.Do(req)
			if resp != nil {
				data, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				_ = json.Unmarshal(data, &job)
			}
			t.Fatalf("timeout waiting for job. Last: status=%s completed=%d/%d failed=%d",
				job.Status, job.CompletedPairs, job.TotalPairs, job.FailedPairs)
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Logf("poll: %v", err)
				continue
			}
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Logf("poll status %d: %s", resp.StatusCode, data)
				continue
			}
			var job liveJobStatus
			if err := json.Unmarshal(data, &job); err != nil {
				t.Logf("decode: %v", err)
				continue
			}
			if job.Status == "complete" || job.Status == "failed" {
				return &job
			}
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
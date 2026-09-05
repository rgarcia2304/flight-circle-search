package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rgarcia2304/flight-circle-search/internal/db"
	"github.com/rgarcia2304/flight-circle-search/internal/fareprovider"
	"github.com/rgarcia2304/flight-circle-search/internal/jobengine"
	"github.com/rgarcia2304/flight-circle-search/internal/worker"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const (
	defaultQueueName     = "search"
	defaultRPS          = 8.0
	defaultMaxConcurrent = 10
)

func main() {
	dbURL := flag.String("db-url", envOr("DATABASE_URL", "postgres://dev:dev@localhost:5432/flightsearch"),
		"Postgres connection string")
	provider := flag.String("provider", envOr("FARE_PROVIDER", "travelpayouts"),
		"fare provider: 'travelpayouts' or 'duffel'")
	duffelToken := flag.String("duffel-token", envOr("DUFFEL_API_TOKEN", ""),
		"Duffel API token (or set DUFFEL_API_TOKEN env var)")
	tpToken := flag.String("tp-token", envOr("TRAVELPAYOUTS_API_KEY", ""),
		"Travelpayouts API token (or set TRAVELPAYOUTS_API_KEY env var)")
	rps := flag.Float64("rps", defaultRPS, "max requests per second to provider API per worker")
	maxConcurrent := flag.Int("max-concurrent", defaultMaxConcurrent, "max concurrent in-flight API calls per worker")
	queueWorkers := flag.Int("queue-workers", 1, "number of concurrent River queue workers per instance")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("-db-url or DATABASE_URL is required")
	}

	var fp fareprovider.FareProvider
	switch *provider {
	case "duffel":
		if *duffelToken == "" {
			log.Fatal("DUFFEL_API_TOKEN is required for --provider=duffel")
		}
		fp = fareprovider.NewDuffel(*duffelToken, "", nil)
	case "travelpayouts":
		if *tpToken == "" {
			log.Fatal("TRAVELPAYOUTS_API_KEY is required for --provider=travelpayouts")
		}
		fp = fareprovider.NewTravelpayouts(*tpToken, "", nil)
	default:
		log.Fatalf("unknown provider %q (want 'duffel' or 'travelpayouts')", *provider)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	if err := db.RunRiverMigrations(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	repo := jobengine.NewPostgresRepository(pool)
	rateLimiter := worker.NewRateLimiter(*rps, *maxConcurrent)
	fareWorker := worker.NewFareWorker(repo, fp, rateLimiter)

	riverWorkers := river.NewWorkers()
	river.AddWorker(riverWorkers, fareWorker)

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			defaultQueueName: {
				MaxWorkers: *queueWorkers,
			},
		},
		Workers: riverWorkers,
	})
	if err != nil {
		log.Fatalf("create river client: %v", err)
	}

	if err := riverClient.Start(ctx); err != nil {
		log.Fatalf("start river client: %v", err)
	}

	log.Printf("worker started: provider=%s rps=%.1f max_concurrent=%d queue_workers=%d",
		*provider, *rps, *maxConcurrent, *queueWorkers)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := riverClient.Stop(shutdownCtx); err != nil {
		log.Printf("river shutdown error: %v", err)
	}
	log.Println("worker stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

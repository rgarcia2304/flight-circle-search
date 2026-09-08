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

func main() {
	dbURL := flag.String("db-url", envOr("DATABASE_URL", "postgres://dev:dev@localhost:5432/flightsearch"),
		"Postgres connection string")
	flag.Parse()

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
	fp := newFareProvider()
	rl := worker.NewRateLimiter(2, 4) // 2 RPS, max 4 in-flight — be gentle to avoid provider rate-limit

	fw := worker.NewFareWorker(repo, fp, rl)

	workers := river.NewWorkers()
	river.AddWorker(workers, fw)

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{"search": {MaxWorkers: 4}},
		Workers: workers,
		Schema:  "public",
	})
	if err != nil {
		log.Fatalf("create river client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		log.Fatalf("start river client: %v", err)
	}

	log.Println("worker started — listening on queue 'search'")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down worker…")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := client.Stop(shutdownCtx); err != nil {
		log.Fatalf("stop river client: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newFareProvider() fareprovider.FareProvider {
	if key := os.Getenv("TRAVELPAYOUTS_API_KEY"); key != "" {
		return fareprovider.NewTravelpayouts(key, "", nil)
	}
	log.Println("warning: TRAVELPAYOUTS_API_KEY not set; worker will fail on all fare searches")
	return fareprovider.NewTravelpayouts("", "", nil)
}

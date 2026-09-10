package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/rgarcia2304/flight-circle-search/internal/cache"
	"github.com/rgarcia2304/flight-circle-search/internal/config"
	"github.com/rgarcia2304/flight-circle-search/internal/db"
	"github.com/rgarcia2304/flight-circle-search/internal/fareprovider"
	"github.com/rgarcia2304/flight-circle-search/internal/jobengine"
)

const searchQueue = "search"

// Runner owns the fare-search worker's dependencies and its River client.
type Runner struct {
	pool       *pgxpool.Pool
	client     *river.Client[pgx.Tx]
	fareCache  *cache.RedisCache
	addr       string
	httpServer *http.Server
}

// NewRunner builds the fare-search worker: Postgres + River, the fare
// provider (with its Redis-backed cache wired in), and the rate limiter.
func NewRunner(cfg config.Config) (*Runner, error) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := db.RunRiverMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	fareCache, err := cache.NewRedisCache(redisOpts.Addr)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	repo := jobengine.NewPostgresRepository(pool)
	fp := fareprovider.NewTravelpayouts(cfg.TravelpayoutsAPIKey, "", nil)
	if cfg.TravelpayoutsAPIKey == "" {
		log.Println("warning: TRAVELPAYOUTS_API_KEY not set; worker will fail on all fare searches")
	}
	fp.SetCache(fareCache, time.Duration(cfg.CacheTTLHours)*time.Hour)

	rl := NewRateLimiter(2, 4) // 2 RPS, max 4 in-flight — be gentle to avoid provider rate-limit
	fw := NewFareWorker(repo, fp, rl)

	workers := river.NewWorkers()
	river.AddWorker(workers, fw)

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{searchQueue: {MaxWorkers: 4}},
		Workers: workers,
		Schema:  "public",
	})
	if err != nil {
		_ = fareCache.Close()
		pool.Close()
		return nil, fmt.Errorf("create river client: %w", err)
	}

	return &Runner{pool: pool, client: client, fareCache: fareCache, addr: cfg.APIAddr}, nil
}

// Run starts processing the search queue and blocks until the process
// receives SIGINT/SIGTERM or ctx is cancelled, then shuts down gracefully.
//
// It also starts a trivial HTTP health server. The worker itself never
// serves HTTP traffic — this exists solely because Cloud Run Services
// require the container to bind $PORT and pass a startup probe, even for
// a pure background poller like this one.
func (r *Runner) Run(ctx context.Context) error {
	r.httpServer = &http.Server{
		Addr:              r.addr,
		Handler:           http.HandlerFunc(healthHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := r.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("health server error: %v", err)
		}
	}()

	if err := r.client.Start(ctx); err != nil {
		return fmt.Errorf("start river client: %w", err)
	}
	log.Println("worker started — listening on queue 'search'")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
	case <-ctx.Done():
	}

	log.Println("shutting down worker…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = r.httpServer.Shutdown(shutdownCtx)
	if err := r.client.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("stop river client: %w", err)
	}
	return nil
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Close releases the runner's Postgres and Redis connections.
func (r *Runner) Close() error {
	r.pool.Close()
	return r.fareCache.Close()
}

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"

	"github.com/rgarcia2304/flight-circle-search/internal/auth"
	"github.com/rgarcia2304/flight-circle-search/internal/config"
	"github.com/rgarcia2304/flight-circle-search/internal/db"
	"github.com/rgarcia2304/flight-circle-search/internal/email"
	authhttp "github.com/rgarcia2304/flight-circle-search/internal/http"
	"github.com/rgarcia2304/flight-circle-search/internal/jobengine"
)

// Server owns every dependency the API process needs and the http.Handler
// built from them.
type Server struct {
	addr        string
	handler     http.Handler
	httpServer  *http.Server
	pool        *pgxpool.Pool
	riverClient *river.Client[pgx.Tx]
	redisClient *redis.Client
}

// NewServer builds the API's full dependency graph (Postgres, River,
// Redis-backed auth, the job engine) and wires the HTTP routes + middleware
// chain. It does not start listening — call Listen for that.
func NewServer(cfg config.Config) (*Server, error) {
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
	redisClient := redis.NewClient(redisOpts)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	riverClient, err := newRiverClient(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}

	repo := jobengine.NewPostgresRepository(pool)
	airports := newCSVAirportSource()
	enqueuer := &riverEnqueuer{client: riverClient}
	svc := jobengine.NewService(repo, airports, enqueuer)

	tokens := auth.NewTokenStore(redisClient, cfg.RateLimitPerDay)
	sessions := auth.NewSessionStore(redisClient)
	sender := email.NewSender(&cfg)
	authHandlers := authhttp.NewAuthHandlers(tokens, sessions, sender, cfg.AppOrigin)

	mux := http.NewServeMux()
	mux.Handle("POST /jobs", authhttp.RequireAuth(sessions)(makeSubmitHandler(svc)))
	mux.Handle("GET /jobs/{id}", authhttp.RequireAuth(sessions)(makeGetHandler(repo)))
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler(pool, redisClient))
	mux.HandleFunc("POST /v1/auth/magic-link", authHandlers.RequestMagicLink)
	mux.HandleFunc("GET /v1/auth/callback", authHandlers.Callback)
	mux.HandleFunc("POST /v1/auth/logout", authHandlers.Logout)

	var handler http.Handler = mux
	handler = authhttp.CORS(cfg.AppOrigin)(handler)
	handler = authhttp.SecurityHeaders(cfg.AppOrigin)(handler)
	handler = authhttp.AccessLog(handler)
	handler = authhttp.RequestID(handler)

	return &Server{
		addr:        cfg.APIAddr,
		handler:     handler,
		pool:        pool,
		riverClient: riverClient,
		redisClient: redisClient,
	}, nil
}

// Handler returns the fully wrapped mux, for tests that want to drive the
// server via httptest without binding a real port.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Listen starts serving on the configured address and blocks until the
// process receives SIGINT/SIGTERM, then shuts down gracefully.
func (s *Server) Listen(ctx context.Context) error {
	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
		case <-ctx.Done():
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
	}()

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// Close releases the server's Postgres and Redis connections.
func (s *Server) Close() error {
	s.pool.Close()
	return s.redisClient.Close()
}

package db

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultConnectTimeout = 5 * time.Second
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed river_migrations/*.sql
var riverMigrationsFS embed.FS

// Config holds Postgres connection settings.
type Config struct {
	URL            string
	MaxConns       int32
	ConnectTimeout time.Duration
	MigrationPath  string
}

// NewPool creates a configured pgxpool.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("db: empty url")
	}
	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if cfg.ConnectTimeout > 0 {
		pcfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrate runs all *.up.sql files in migrations/ in lexical order.
// Idempotent: re-running a migration is a no-op for CREATE TABLE IF NOT EXISTS-compatible statements.
// We use a schema_migrations table to track applied versions.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			dirty BOOLEAN NOT NULL DEFAULT false,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var ups []string
	for _, e := range entries {
		name := e.Name()
		if len(name) > 7 && name[len(name)-7:] == ".up.sql" {
			ups = append(ups, name)
		}
	}
	sort.Strings(ups)

	for _, name := range ups {
		version := name[:len(name)-7] // strip ".up.sql"
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
			version,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check %s: %w", version, err)
		}
		if exists {
			continue
		}

		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations(version, dirty) VALUES ($1, false)`,
			version,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
	}
	return nil
}

// RunRiverMigrations runs River's schema migrations (river_queue, river_job, etc.).
// Calls Migrate first, then River's migrations.
func RunRiverMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if err := Migrate(ctx, pool); err != nil {
		return err
	}

	entries, err := riverMigrationsFS.ReadDir("river_migrations")
	if err != nil {
		return fmt.Errorf("read river migrations: %w", err)
	}
	var ups []string
	for _, e := range entries {
		name := e.Name()
		if len(name) > 7 && name[len(name)-7:] == ".up.sql" {
			ups = append(ups, name)
		}
	}
	sort.Strings(ups)

	for _, name := range ups {
		version := name[:len(name)-7]
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
			version,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check river migration %s: %w", version, err)
		}
		if exists {
			continue
		}

		body, err := riverMigrationsFS.ReadFile("river_migrations/" + name)
		if err != nil {
			return fmt.Errorf("read river migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for river %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply river %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations(version, dirty) VALUES ($1, false)`,
			version,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record river %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit river %s: %w", name, err)
		}
	}
	return nil
}

// Package db manages the PostgreSQL connection pool and schema migrations.
package db

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/config"
)

// Connect creates and validates a pgxpool connection pool.
// The pool is safe for concurrent use and should be shared across the application.
func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

// Migrate applies any pending up-migrations from the embedded SQL files.
// It is idempotent: calling it when no migrations are pending is a no-op.
// migrationsFS must be the embed.FS containing the migrations directory.
func Migrate(dbURL string, migrationsFS fs.FS) (retErr error) {
	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("initializing migrator: %w", err)
	}
	defer func() {
		_, closeErr := m.Close()
		if retErr == nil && closeErr != nil {
			retErr = fmt.Errorf("closing migrator: %w", closeErr)
		}
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/auth"
)

// WithSystemScope executes fn within a database transaction that has
// app.current_system_id set via SET LOCAL (using the parameterized
// set_config function to prevent SQL injection).
//
// When systemID is empty (platform admin), the session variable is not
// set, leaving RLS in admin mode (empty = unrestricted access).
//
// The transaction is committed if fn returns nil; rolled back otherwise.
func WithSystemScope(ctx context.Context, pool *pgxpool.Pool, systemID string, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if systemID != "" {
		// set_config with third arg true = LOCAL (scoped to this transaction).
		// Parameterized: no SQL injection risk from systemID.
		if _, err := tx.Exec(ctx, "SELECT set_config('app.current_system_id', $1, true)", systemID); err != nil {
			return fmt.Errorf("setting system scope: %w", err)
		}
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// WithAuthScope extracts the system_id from the authenticated identity in
// the request context and delegates to WithSystemScope. This is the primary
// entry point for handlers that need RLS-scoped database access.
func WithAuthScope(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	identity := auth.MustIdentityFromContext(ctx)
	return WithSystemScope(ctx, pool, identity.SystemID, fn)
}

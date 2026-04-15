package overlay

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts the persistence layer for property overlays.
// Write methods accept pgx.Tx to guarantee atomicity with audit log writes.
// Read methods accept db.DBTX to work both inside and outside transactions.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, o *PropertyOverlay) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*PropertyOverlay, error)
	GetByEntityAndSystem(ctx context.Context, q db.DBTX, entityID, systemID uuid.UUID) (*PropertyOverlay, error)
	ListByEntity(ctx context.Context, q db.DBTX, entityID uuid.UUID, params ListParams) ([]*PropertyOverlay, int64, error)
	Update(ctx context.Context, tx pgx.Tx, o *PropertyOverlay) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

func scanOverlay(row pgx.Row) (*PropertyOverlay, error) {
	o := &PropertyOverlay{}
	err := row.Scan(&o.ID, &o.EntityID, &o.SystemID, &o.Properties, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning property overlay: %w", err)
	}
	return o, nil
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, o *PropertyOverlay) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO property_overlay (entity_id, system_id, properties)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		o.EntityID, o.SystemID, o.Properties,
	).Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return ErrDuplicate
			case "23503":
				// FK on entity_id
				return ErrEntityNotFound
			}
		}
		return fmt.Errorf("inserting property overlay: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*PropertyOverlay, error) {
	row := q.QueryRow(ctx,
		`SELECT id, entity_id, system_id, properties, created_at, updated_at
		 FROM property_overlay
		 WHERE id = $1`,
		id,
	)
	return scanOverlay(row)
}

func (r *pgxRepository) GetByEntityAndSystem(ctx context.Context, q db.DBTX, entityID, systemID uuid.UUID) (*PropertyOverlay, error) {
	row := q.QueryRow(ctx,
		`SELECT id, entity_id, system_id, properties, created_at, updated_at
		 FROM property_overlay
		 WHERE entity_id = $1 AND system_id = $2`,
		entityID, systemID,
	)
	return scanOverlay(row)
}

func (r *pgxRepository) ListByEntity(ctx context.Context, q db.DBTX, entityID uuid.UUID, params ListParams) ([]*PropertyOverlay, int64, error) {
	var total int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM property_overlay WHERE entity_id = $1`, entityID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting property overlays: %w", err)
	}

	rows, err := q.Query(ctx,
		`SELECT id, entity_id, system_id, properties, created_at, updated_at
		 FROM property_overlay
		 WHERE entity_id = $1
		 ORDER BY system_id
		 LIMIT $2 OFFSET $3`,
		entityID, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("querying property overlays: %w", err)
	}
	defer rows.Close()

	var result []*PropertyOverlay
	for rows.Next() {
		o := &PropertyOverlay{}
		if err := rows.Scan(&o.ID, &o.EntityID, &o.SystemID, &o.Properties, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning property overlay: %w", err)
		}
		result = append(result, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating property overlays: %w", err)
	}
	return result, total, nil
}

func (r *pgxRepository) Update(ctx context.Context, tx pgx.Tx, o *PropertyOverlay) error {
	err := tx.QueryRow(ctx,
		`UPDATE property_overlay SET properties = $1 WHERE id = $2 RETURNING updated_at`,
		o.Properties, o.ID,
	).Scan(&o.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("updating property overlay: %w", err)
	}
	return nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	tag, err := tx.Exec(ctx, `DELETE FROM property_overlay WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting property overlay: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

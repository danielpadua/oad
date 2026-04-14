package overlayschema

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts the persistence layer for system overlay schemas.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, schema *SystemOverlaySchema) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*SystemOverlaySchema, error)
	ListBySystem(ctx context.Context, q db.DBTX, systemID uuid.UUID) ([]*SystemOverlaySchema, error)
	Update(ctx context.Context, tx pgx.Tx, schema *SystemOverlaySchema) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
	HasOverlays(ctx context.Context, q db.DBTX, id uuid.UUID) (bool, error)
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, schema *SystemOverlaySchema) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO system_overlay_schema
		    (system_id, entity_type_id, allowed_overlay_properties)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		schema.SystemID, schema.EntityTypeID, schema.AllowedOverlayProperties,
	).Scan(&schema.ID, &schema.CreatedAt, &schema.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return ErrDuplicate
			case "23503":
				// Disambiguate FK violation by constraint name.
				if pgErr.ConstraintName == "system_overlay_schema_system_id_fkey" {
					return ErrSystemNotFound
				}
				return ErrEntityTypeNotFound
			}
		}
		return fmt.Errorf("inserting system overlay schema: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*SystemOverlaySchema, error) {
	schema := &SystemOverlaySchema{}
	err := q.QueryRow(ctx,
		`SELECT id, system_id, entity_type_id, allowed_overlay_properties, created_at, updated_at
		 FROM system_overlay_schema
		 WHERE id = $1`,
		id,
	).Scan(&schema.ID, &schema.SystemID, &schema.EntityTypeID,
		&schema.AllowedOverlayProperties, &schema.CreatedAt, &schema.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying system overlay schema: %w", err)
	}
	return schema, nil
}

func (r *pgxRepository) ListBySystem(ctx context.Context, q db.DBTX, systemID uuid.UUID) ([]*SystemOverlaySchema, error) {
	rows, err := q.Query(ctx,
		`SELECT id, system_id, entity_type_id, allowed_overlay_properties, created_at, updated_at
		 FROM system_overlay_schema
		 WHERE system_id = $1
		 ORDER BY entity_type_id`,
		systemID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying system overlay schemas: %w", err)
	}
	defer rows.Close()

	var result []*SystemOverlaySchema
	for rows.Next() {
		schema := &SystemOverlaySchema{}
		if err := rows.Scan(&schema.ID, &schema.SystemID, &schema.EntityTypeID,
			&schema.AllowedOverlayProperties, &schema.CreatedAt, &schema.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning system overlay schema: %w", err)
		}
		result = append(result, schema)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating system overlay schemas: %w", err)
	}
	return result, nil
}

func (r *pgxRepository) Update(ctx context.Context, tx pgx.Tx, schema *SystemOverlaySchema) error {
	err := tx.QueryRow(ctx,
		`UPDATE system_overlay_schema
		 SET allowed_overlay_properties = $1
		 WHERE id = $2
		 RETURNING updated_at`,
		schema.AllowedOverlayProperties, schema.ID,
	).Scan(&schema.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("updating system overlay schema: %w", err)
	}
	return nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM system_overlay_schema WHERE id = $1`, id,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrHasOverlays
		}
		return fmt.Errorf("deleting system overlay schema: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgxRepository) HasOverlays(ctx context.Context, q db.DBTX, id uuid.UUID) (bool, error) {
	// property_overlay links to a schema implicitly via (entity_id, system_id).
	// We check by joining overlay schema → property_overlay through system_id and entity type.
	var count int64
	err := q.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM property_overlay po
		 JOIN entity e ON e.id = po.entity_id
		 JOIN system_overlay_schema sos ON sos.system_id = po.system_id
		     AND sos.entity_type_id = e.type_id
		 WHERE sos.id = $1`,
		id,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("counting overlays for schema: %w", err)
	}
	return count > 0, nil
}

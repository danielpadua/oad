package entitytype

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts the persistence layer for entity type definitions.
// Write methods accept pgx.Tx to guarantee atomicity with audit log writes.
// Read methods accept db.DBTX to work both inside and outside transactions.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, etd *EntityTypeDefinition) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*EntityTypeDefinition, error)
	List(ctx context.Context, q db.DBTX) ([]*EntityTypeDefinition, error)
	Update(ctx context.Context, tx pgx.Tx, etd *EntityTypeDefinition) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
	HasEntities(ctx context.Context, q db.DBTX, id uuid.UUID) (bool, error)
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, etd *EntityTypeDefinition) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO entity_type_definition
		    (type_name, allowed_properties, allowed_relations, scope)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		etd.TypeName, etd.AllowedProperties, etd.AllowedRelations, etd.Scope,
	).Scan(&etd.ID, &etd.CreatedAt, &etd.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateTypeName
		}
		return fmt.Errorf("inserting entity type definition: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*EntityTypeDefinition, error) {
	etd := &EntityTypeDefinition{}
	err := q.QueryRow(ctx,
		`SELECT id, type_name, allowed_properties, allowed_relations, scope, created_at, updated_at
		 FROM entity_type_definition
		 WHERE id = $1`,
		id,
	).Scan(&etd.ID, &etd.TypeName, &etd.AllowedProperties, &etd.AllowedRelations,
		&etd.Scope, &etd.CreatedAt, &etd.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying entity type definition: %w", err)
	}
	return etd, nil
}

func (r *pgxRepository) List(ctx context.Context, q db.DBTX) ([]*EntityTypeDefinition, error) {
	rows, err := q.Query(ctx,
		`SELECT id, type_name, allowed_properties, allowed_relations, scope, created_at, updated_at
		 FROM entity_type_definition
		 ORDER BY type_name`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying entity type definitions: %w", err)
	}
	defer rows.Close()

	var result []*EntityTypeDefinition
	for rows.Next() {
		etd := &EntityTypeDefinition{}
		if err := rows.Scan(&etd.ID, &etd.TypeName, &etd.AllowedProperties, &etd.AllowedRelations,
			&etd.Scope, &etd.CreatedAt, &etd.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning entity type definition: %w", err)
		}
		result = append(result, etd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating entity type definitions: %w", err)
	}
	return result, nil
}

func (r *pgxRepository) Update(ctx context.Context, tx pgx.Tx, etd *EntityTypeDefinition) error {
	err := tx.QueryRow(ctx,
		`UPDATE entity_type_definition
		 SET allowed_properties = $1, allowed_relations = $2
		 WHERE id = $3
		 RETURNING updated_at`,
		etd.AllowedProperties, etd.AllowedRelations, etd.ID,
	).Scan(&etd.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("updating entity type definition: %w", err)
	}
	return nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM entity_type_definition WHERE id = $1`, id,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		// FK violation: entities reference this type (defense-in-depth, service pre-checks).
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrHasEntities
		}
		return fmt.Errorf("deleting entity type definition: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgxRepository) HasEntities(ctx context.Context, q db.DBTX, id uuid.UUID) (bool, error) {
	var count int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM entity WHERE type_id = $1`, id,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("counting entities for type: %w", err)
	}
	return count > 0, nil
}

package entity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts the persistence layer for entities.
// Write methods accept pgx.Tx to guarantee atomicity with audit log writes.
// Read methods accept db.DBTX to work both inside and outside transactions.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, e *Entity) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*Entity, error)
	GetByTypeAndExternalID(ctx context.Context, q db.DBTX, typeID uuid.UUID, externalID string) (*Entity, error)
	List(ctx context.Context, q db.DBTX, params ListParams) ([]*Entity, int64, error)
	Update(ctx context.Context, tx pgx.Tx, e *Entity) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

func scanEntity(row pgx.Row) (*Entity, error) {
	e := &Entity{}
	err := row.Scan(
		&e.ID, &e.TypeID, &e.Type, &e.ExternalID,
		&e.Properties, &e.SystemID, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning entity: %w", err)
	}
	return e, nil
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, e *Entity) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO entity (type_id, external_id, properties, system_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		e.TypeID, e.ExternalID, e.Properties, e.SystemID,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateExternalID
		}
		return fmt.Errorf("inserting entity: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*Entity, error) {
	row := q.QueryRow(ctx,
		`SELECT e.id, e.type_id, etd.type_name, e.external_id, e.properties, e.system_id, e.created_at, e.updated_at
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE e.id = $1`,
		id,
	)
	return scanEntity(row)
}

func (r *pgxRepository) GetByTypeAndExternalID(ctx context.Context, q db.DBTX, typeID uuid.UUID, externalID string) (*Entity, error) {
	row := q.QueryRow(ctx,
		`SELECT e.id, e.type_id, etd.type_name, e.external_id, e.properties, e.system_id, e.created_at, e.updated_at
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE e.type_id = $1 AND e.external_id = $2`,
		typeID, externalID,
	)
	return scanEntity(row)
}

func (r *pgxRepository) List(ctx context.Context, q db.DBTX, params ListParams) ([]*Entity, int64, error) {
	args := []any{}
	conds := []string{}

	if params.TypeName != "" {
		args = append(args, params.TypeName)
		conds = append(conds, fmt.Sprintf("etd.type_name = $%d", len(args)))
	}

	whereClause := "TRUE"
	if len(conds) > 0 {
		whereClause = strings.Join(conds, " AND ")
	}

	baseFrom := `FROM entity e JOIN entity_type_definition etd ON etd.id = e.type_id WHERE ` + whereClause

	var total int64
	if err := q.QueryRow(ctx, "SELECT COUNT(*) "+baseFrom, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting entities: %w", err)
	}

	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, params.Limit, params.Offset)

	listSQL := fmt.Sprintf(
		`SELECT e.id, e.type_id, etd.type_name, e.external_id, e.properties, e.system_id, e.created_at, e.updated_at
		 %s ORDER BY e.created_at DESC LIMIT $%d OFFSET $%d`,
		baseFrom, len(listArgs)-1, len(listArgs),
	)

	rows, err := q.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying entities: %w", err)
	}
	defer rows.Close()

	result := []*Entity{}
	for rows.Next() {
		e := &Entity{}
		if err := rows.Scan(&e.ID, &e.TypeID, &e.Type, &e.ExternalID,
			&e.Properties, &e.SystemID, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning entity: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating entities: %w", err)
	}
	return result, total, nil
}

func (r *pgxRepository) Update(ctx context.Context, tx pgx.Tx, e *Entity) error {
	err := tx.QueryRow(ctx,
		`UPDATE entity SET properties = $1 WHERE id = $2 RETURNING updated_at`,
		e.Properties, e.ID,
	).Scan(&e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("updating entity: %w", err)
	}
	return nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	tag, err := tx.Exec(ctx, `DELETE FROM entity WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting entity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

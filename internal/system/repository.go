package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// systemTypeName is the type_name of the built-in entity type that
// represents a registered system. Seeded by migration 000001.
const systemTypeName = "System"

// Repository abstracts the persistence layer for systems. Systems are
// persisted as entities of type 'System' (see migration 000001 and
// docs/design/scim-ingest.md §3.4): one row in `entity` per system, with
// `external_id` carrying the system name and `properties` carrying
// `{name, description, active}`.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, s *System) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*System, error)
	List(ctx context.Context, q db.DBTX, allowedIDs []uuid.UUID) ([]*System, error)
	Update(ctx context.Context, tx pgx.Tx, s *System) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

// systemProperties is the JSON shape stored in entity.properties for
// rows of type System. Mirrors the JSON Schema seeded by migration 000001.
type systemProperties struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, s *System) error {
	s.Active = true

	props, err := json.Marshal(systemProperties{
		Name:        s.Name,
		Description: s.Description,
		Active:      s.Active,
	})
	if err != nil {
		return fmt.Errorf("marshalling system properties: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO entity (type_id, external_id, properties)
		 SELECT id, $1, $2::jsonb FROM entity_type_definition WHERE type_name = $3
		 RETURNING id, created_at, updated_at`,
		s.Name, props, systemTypeName,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateName
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("built-in System type not seeded: %w", err)
		}
		return fmt.Errorf("inserting system entity: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*System, error) {
	s := &System{}
	err := q.QueryRow(ctx,
		`SELECT e.id,
		        COALESCE(e.properties->>'name', '')        AS name,
		        COALESCE(e.properties->>'description', '') AS description,
		        COALESCE((e.properties->>'active')::bool, true) AS active,
		        e.created_at,
		        e.updated_at
		 FROM entity e
		 JOIN entity_type_definition t ON t.id = e.type_id
		 WHERE e.id = $1 AND t.type_name = $2`,
		id, systemTypeName,
	).Scan(&s.ID, &s.Name, &s.Description, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying system: %w", err)
	}
	return s, nil
}

func (r *pgxRepository) List(ctx context.Context, q db.DBTX, allowedIDs []uuid.UUID) ([]*System, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if len(allowedIDs) > 0 {
		rows, err = q.Query(ctx,
			`SELECT e.id,
			        COALESCE(e.properties->>'name', '')        AS name,
			        COALESCE(e.properties->>'description', '') AS description,
			        COALESCE((e.properties->>'active')::bool, true) AS active,
			        e.created_at,
			        e.updated_at
			 FROM entity e
			 JOIN entity_type_definition t ON t.id = e.type_id
			 WHERE t.type_name = $1
			   AND e.id = ANY($2)
			 ORDER BY e.properties->>'name'`,
			systemTypeName, allowedIDs,
		)
	} else {
		rows, err = q.Query(ctx,
			`SELECT e.id,
			        COALESCE(e.properties->>'name', '')        AS name,
			        COALESCE(e.properties->>'description', '') AS description,
			        COALESCE((e.properties->>'active')::bool, true) AS active,
			        e.created_at,
			        e.updated_at
			 FROM entity e
			 JOIN entity_type_definition t ON t.id = e.type_id
			 WHERE t.type_name = $1
			 ORDER BY e.properties->>'name'`,
			systemTypeName,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("querying systems: %w", err)
	}
	defer rows.Close()

	var result []*System
	for rows.Next() {
		s := &System{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Active,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning system: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating systems: %w", err)
	}
	return result, nil
}

func (r *pgxRepository) Update(ctx context.Context, tx pgx.Tx, s *System) error {
	props, err := json.Marshal(systemProperties{
		Name:        s.Name,
		Description: s.Description,
		Active:      s.Active,
	})
	if err != nil {
		return fmt.Errorf("marshalling system properties: %w", err)
	}

	err = tx.QueryRow(ctx,
		`UPDATE entity
		 SET external_id = $1,
		     properties  = $2::jsonb
		 WHERE id = $3
		   AND type_id = (SELECT id FROM entity_type_definition WHERE type_name = $4)
		 RETURNING updated_at`,
		s.Name, props, s.ID, systemTypeName,
	).Scan(&s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateName
		}
		return fmt.Errorf("updating system: %w", err)
	}
	return nil
}

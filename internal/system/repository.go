package system

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts the persistence layer for systems.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, s *System) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*System, error)
	List(ctx context.Context, q db.DBTX) ([]*System, error)
	Update(ctx context.Context, tx pgx.Tx, s *System) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, s *System) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO system (name, description)
		 VALUES ($1, $2)
		 RETURNING id, active, created_at, updated_at`,
		s.Name, s.Description,
	).Scan(&s.ID, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateName
		}
		return fmt.Errorf("inserting system: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*System, error) {
	s := &System{}
	err := q.QueryRow(ctx,
		`SELECT id, name, description, active, created_at, updated_at
		 FROM system
		 WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.Name, &s.Description, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying system: %w", err)
	}
	return s, nil
}

func (r *pgxRepository) List(ctx context.Context, q db.DBTX) ([]*System, error) {
	rows, err := q.Query(ctx,
		`SELECT id, name, description, active, created_at, updated_at
		 FROM system
		 ORDER BY name`,
	)
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
	err := tx.QueryRow(ctx,
		`UPDATE system
		 SET name = $1, description = $2, active = $3
		 WHERE id = $4
		 RETURNING updated_at`,
		s.Name, s.Description, s.Active, s.ID,
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

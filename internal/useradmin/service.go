package useradmin

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service coordinates user admin operations.
type Service struct {
	db   *pgxpool.Pool
	repo Repository
}

// NewService creates a new Service.
func NewService(db *pgxpool.Pool, repo Repository) *Service {
	return &Service{
		db:   db,
		repo: repo,
	}
}

// List returns a paginated list of users including their external identities.
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	result, err := s.repo.List(ctx, s.db, params)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	return result, nil
}

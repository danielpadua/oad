package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/auth"
	"github.com/danielpadua/oad/internal/db"
)

// Service orchestrates business logic for system management.
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService creates a new system service.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// Create registers a new system (FR-SYS-001).
func (s *Service) Create(ctx context.Context, req CreateRequest) (*System, error) {
	if req.Name == "" {
		return nil, apierr.BadRequest("name is required")
	}

	sys := &System{
		Name:        req.Name,
		Description: req.Description,
	}

	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, sys); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(sys)
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpCreate,
			ResourceType: "system",
			ResourceID:   sys.ID,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return sys, nil
}

// GetByID retrieves a single system by its UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*System, error) {
	sys, err := s.repo.GetByID(ctx, s.pool, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return sys, nil
}

// List returns all systems ordered by name.
func (s *Service) List(ctx context.Context) ([]*System, error) {
	items, err := s.repo.List(ctx, s.pool)
	if err != nil {
		return nil, fmt.Errorf("listing systems: %w", err)
	}
	if items == nil {
		items = []*System{}
	}
	return items, nil
}

// Patch applies partial updates to a system (FR-SYS-002, FR-SYS-003).
// Setting Active to false deactivates the system without deleting its data.
//
// Authorization: platform admins (unscoped identities) may change any field.
// System-scoped admins are restricted to the `description` field of their own
// system — name changes and activation toggles are reserved for platform admins.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, req PatchRequest) (*System, error) {
	if req.Name != nil && *req.Name == "" {
		return nil, apierr.BadRequest("name cannot be empty")
	}

	if identity, ok := auth.IdentityFromContext(ctx); ok && identity.SystemID != "" {
		if identity.SystemID != id.String() {
			return nil, apierr.Forbidden("access denied to system " + id.String())
		}
		if req.Name != nil || req.Active != nil {
			return nil, apierr.Forbidden("only platform admins may change name or active state")
		}
	}

	var result *System
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}
		beforeJSON, _ := json.Marshal(existing)

		if req.Name != nil {
			existing.Name = *req.Name
		}
		if req.Description != nil {
			existing.Description = *req.Description
		}
		if req.Active != nil {
			existing.Active = *req.Active
		}

		if err := s.repo.Update(ctx, tx, existing); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(existing)
		result = existing
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpUpdate,
			ResourceType: "system",
			ResourceID:   existing.ID,
			BeforeValue:  beforeJSON,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return result, nil
}

// toAPIError converts domain and repository errors to *apierr.APIError.
func toAPIError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *apierr.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apierr.NotFound("system")
	case errors.Is(err, ErrDuplicateName):
		return apierr.Conflict("system name already exists")
	}
	return err
}

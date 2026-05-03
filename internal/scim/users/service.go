package users

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/audit"
)

// Service orchestrates SCIM User operations. Each method runs inside its
// own transaction so the entity, entity_external_identity, and audit_log
// writes commit atomically (NFR-AUD-001 — no mutation without audit).
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService returns a Service backed by the given pool and dependencies.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// scimActor builds the audit_log.actor for SCIM operations: "scim:<provider>".
func scimActor(providerName string) string {
	return "scim:" + providerName
}

// Create provisions a new User from an inbound SCIM payload. Returns the
// stored User as it should be rendered back to the caller (id, etag, meta
// populated).
func (s *Service) Create(ctx context.Context, providerName string, in User) (*User, error) {
	if in.UserName == "" {
		return nil, ErrUserNameRequired
	}
	if in.ExternalID == "" {
		return nil, ErrExternalIDRequired
	}

	props := ToProperties(in)
	propsJSON, _ := json.Marshal(props)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	stored, err := s.repo.Create(ctx, tx, providerName, in.ExternalID, props)
	if err != nil {
		return nil, err
	}

	if err := s.audit.Write(ctx, tx, audit.Entry{
		Actor:        scimActor(providerName),
		Operation:    audit.OpCreate,
		ResourceType: "user",
		ResourceID:   stored.EntityID,
		AfterValue:   propsJSON,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	out := FromStored(*stored)
	return &out, nil
}

// GetByEntityID reads the User identified by id within the calling provider's
// tenant. Returns ErrNotFound when the user does not exist or belongs to
// another provider.
func (s *Service) GetByEntityID(ctx context.Context, providerName string, id uuid.UUID) (*User, error) {
	stored, err := s.repo.GetByEntityID(ctx, s.pool, providerName, id)
	if err != nil {
		return nil, err
	}
	out := FromStored(*stored)
	return &out, nil
}

// Delete removes the calling provider's link to the user. If the user has
// no remaining provider links after the delete, the underlying entity is
// also removed (CASCADE handles relations).
func (s *Service) Delete(ctx context.Context, providerName string, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// Read for the audit record; the same query also confirms ownership and
	// returns ErrNotFound when the calling provider has no claim.
	before, err := s.repo.GetByEntityID(ctx, tx, providerName, id)
	if err != nil {
		return err
	}
	beforeJSON, _ := json.Marshal(before.Properties)

	if err := s.repo.Delete(ctx, tx, providerName, id); err != nil {
		return err
	}

	if err := s.audit.Write(ctx, tx, audit.Entry{
		Actor:        scimActor(providerName),
		Operation:    audit.OpDelete,
		ResourceType: "user",
		ResourceID:   id,
		BeforeValue:  beforeJSON,
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

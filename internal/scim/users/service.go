package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/scim/parser"
)

// CacheInvalidator is implemented by auth.IdentityCache.
// It allows the SCIM service to evict stale identity entries after mutations.
type CacheInvalidator interface {
	Invalidate(provider, sub string)
}

// Service orchestrates SCIM User operations. Each method runs inside its
// own transaction so the entity, entity_external_identity, and audit_log
// writes commit atomically (NFR-AUD-001 — no mutation without audit).
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
	cache CacheInvalidator // nil when invalidation not wired
}

// NewService returns a Service backed by the given pool and dependencies.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// SetCacheInvalidator injects a cache invalidation hook. Call once at startup.
func (s *Service) SetCacheInvalidator(inv CacheInvalidator) {
	s.cache = inv
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

// ListResult is the payload returned by Service.List: a page of users
// rendered as SCIM Users plus the total count for pagination.
type ListResult struct {
	Users []*User
	Total int
}

// List returns a paginated view of users belonging to the calling provider.
// filterStr is a SCIM filter expression per RFC 7644 §3.4.2.2 (subset
// supported — see parser package). Empty filterStr returns all users.
//
// offset is 0-based (callers translate startIndex 1-based to offset = N-1).
// limit > 200 is the caller's responsibility to clamp; this method honors
// whatever it is given.
func (s *Service) List(ctx context.Context, providerName, filterStr string, offset, limit int) (*ListResult, error) {
	var (
		filterSQL  string
		filterArgs []any
	)
	if filterStr != "" {
		expr, err := parser.Parse(filterStr)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidFilter, err)
		}
		filterSQL, filterArgs, err = FilterToSQL(expr)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidFilter, err)
		}
	}

	stored, total, err := s.repo.List(ctx, s.pool, providerName, filterSQL, filterArgs, offset, limit)
	if err != nil {
		return nil, err
	}

	users := make([]*User, len(stored))
	for i, su := range stored {
		u := FromStored(*su)
		users[i] = &u
	}
	return &ListResult{Users: users, Total: total}, nil
}

// Replace overwrites the user's stored properties from the inbound SCIM
// payload (PUT semantics per RFC 7644 §3.5.1). The (provider,
// external_subject) link and the entity_id are preserved; only the
// properties JSONB is rewritten.
func (s *Service) Replace(ctx context.Context, providerName string, id uuid.UUID, in User) (*User, error) {
	if in.UserName == "" {
		return nil, ErrUserNameRequired
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	before, err := s.repo.GetByEntityID(ctx, tx, providerName, id)
	if err != nil {
		return nil, err
	}

	newProps := ToProperties(in)
	updatedAt, err := s.repo.Replace(ctx, tx, providerName, id, newProps)
	if err != nil {
		// ErrNotFound here would only happen on a concurrent delete
		// between the GetByEntityID and Replace calls.
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	beforeJSON, _ := json.Marshal(before.Properties)
	afterJSON, _ := json.Marshal(newProps)
	if err := s.audit.Write(ctx, tx, audit.Entry{
		Actor:        scimActor(providerName),
		Operation:    audit.OpUpdate,
		ResourceType: "user",
		ResourceID:   id,
		BeforeValue:  beforeJSON,
		AfterValue:   afterJSON,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.cache != nil {
		s.cache.Invalidate(providerName, before.ExternalSubject)
	}

	out := FromStored(StoredUser{
		EntityID:        id,
		Properties:      newProps,
		ExternalSubject: before.ExternalSubject,
		CreatedAt:       before.CreatedAt,
		UpdatedAt:       updatedAt,
	})
	return &out, nil
}

// Patch applies a SCIM PATCH operation list to the user (RFC 7644
// §3.5.2). The supported subset is documented on ApplyPatch. Behavior
// mirrors Replace: properties are rewritten atomically with an audit
// entry; the (provider, external_subject) link and entity_id are
// preserved.
func (s *Service) Patch(ctx context.Context, providerName string, id uuid.UUID, ops []PatchOp) (*User, error) {
	if len(ops) == 0 {
		return s.GetByEntityID(ctx, providerName, id)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	before, err := s.repo.GetByEntityID(ctx, tx, providerName, id)
	if err != nil {
		return nil, err
	}

	newProps, err := ApplyPatch(before.Properties, ops)
	if err != nil {
		return nil, err
	}

	// Enforce the schema-required attribute even after PATCH: clients
	// must not be able to remove userName via a future op.
	if _, ok := newProps[PropertyKeyUserName].(string); !ok {
		return nil, ErrUserNameRequired
	}

	updatedAt, err := s.repo.Replace(ctx, tx, providerName, id, newProps)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	beforeJSON, _ := json.Marshal(before.Properties)
	afterJSON, _ := json.Marshal(newProps)
	if err := s.audit.Write(ctx, tx, audit.Entry{
		Actor:        scimActor(providerName),
		Operation:    audit.OpUpdate,
		ResourceType: "user",
		ResourceID:   id,
		BeforeValue:  beforeJSON,
		AfterValue:   afterJSON,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.cache != nil {
		s.cache.Invalidate(providerName, before.ExternalSubject)
	}

	out := FromStored(StoredUser{
		EntityID:        id,
		Properties:      newProps,
		ExternalSubject: before.ExternalSubject,
		CreatedAt:       before.CreatedAt,
		UpdatedAt:       updatedAt,
	})
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

	if s.cache != nil {
		s.cache.Invalidate(providerName, before.ExternalSubject)
	}

	return nil
}

package groups

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

// Service orchestrates SCIM Group operations. Each method runs inside its
// own transaction so the entity, entity_external_identity, member
// relations, and audit_log writes commit atomically.
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

// Create provisions a new Group. memberIDs in the inbound payload are
// validated against the calling provider's tenant — every value must
// reference an existing User or Group entity already linked to the same
// provider.
func (s *Service) Create(ctx context.Context, providerName string, in Group) (*Group, error) {
	if in.DisplayName == "" {
		return nil, ErrDisplayNameRequired
	}
	if in.ExternalID == "" {
		return nil, ErrExternalIDRequired
	}

	memberIDs, err := parseMemberIDs(in.Members)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if len(memberIDs) > 0 {
		resolved, err := s.repo.ValidateMembers(ctx, tx, providerName, memberIDs)
		if err != nil {
			return nil, err
		}
		if len(resolved) != len(memberIDs) {
			return nil, fmt.Errorf("%w: one or more member references not found in provider tenant", ErrInvalidMember)
		}
	}

	props := ToProperties(in)
	propsJSON, _ := json.Marshal(props)

	stored, err := s.repo.Create(ctx, tx, providerName, in.ExternalID, props, memberIDs)
	if err != nil {
		return nil, err
	}

	if err := s.audit.Write(ctx, tx, audit.Entry{
		Actor:        scimActor(providerName),
		Operation:    audit.OpCreate,
		ResourceType: "group",
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

// GetByEntityID reads the Group identified by id within the calling
// provider's tenant. Returns ErrNotFound when the group does not exist
// or belongs to another provider.
func (s *Service) GetByEntityID(ctx context.Context, providerName string, id uuid.UUID) (*Group, error) {
	stored, err := s.repo.GetByEntityID(ctx, s.pool, providerName, id)
	if err != nil {
		return nil, err
	}
	out := FromStored(*stored)
	return &out, nil
}

// Delete removes the calling provider's link to the group. If the group
// has no remaining provider links, the entity (and its membership
// relations via cascade) is removed.
func (s *Service) Delete(ctx context.Context, providerName string, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

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
		ResourceType: "group",
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

// ListResult is the payload returned by Service.List: a page of groups
// rendered as SCIM Groups plus the total count for pagination.
type ListResult struct {
	Groups []*Group
	Total  int
}

// List returns a paginated view of groups belonging to the calling
// provider. Members are NOT populated on the rendered groups (the list
// surface is intentionally lightweight); clients fetch a specific Group
// by id when they need its membership.
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

	out := make([]*Group, len(stored))
	for i, sg := range stored {
		g := FromStored(*sg)
		out[i] = &g
	}
	return &ListResult{Groups: out, Total: total}, nil
}

// Replace overwrites the group's stored properties and reconciles its
// membership against the inbound payload (PUT semantics per RFC 7644
// §3.5.1). The (provider, external_subject) link and entity_id are
// preserved; only properties + member_of relations are rewritten.
func (s *Service) Replace(ctx context.Context, providerName string, id uuid.UUID, in Group) (*Group, error) {
	if in.DisplayName == "" {
		return nil, ErrDisplayNameRequired
	}

	memberIDs, err := parseMemberIDs(in.Members)
	if err != nil {
		return nil, err
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

	if len(memberIDs) > 0 {
		resolved, err := s.repo.ValidateMembers(ctx, tx, providerName, memberIDs)
		if err != nil {
			return nil, err
		}
		if len(resolved) != len(memberIDs) {
			return nil, fmt.Errorf("%w: one or more member references not found in provider tenant", ErrInvalidMember)
		}
	}

	newProps := ToProperties(in)
	stored, err := s.repo.Replace(ctx, tx, providerName, id, newProps, memberIDs)
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
		ResourceType: "group",
		ResourceID:   id,
		BeforeValue:  beforeJSON,
		AfterValue:   afterJSON,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	out := FromStored(*stored)
	return &out, nil
}

// Patch applies a SCIM PATCH operation list to the group (RFC 7644
// §3.5.2). The supported subset is documented on ApplyPatch. Behavior
// mirrors Replace: properties + member relations are rewritten
// atomically with an audit entry; the (provider, external_subject)
// link and entity_id are preserved.
func (s *Service) Patch(ctx context.Context, providerName string, id uuid.UUID, ops []PatchOp) (*Group, error) {
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

	state := PatchState{
		Properties: before.Properties,
		MemberIDs:  memberEntityIDs(before.Members),
	}
	next, err := ApplyPatch(state, ops)
	if err != nil {
		return nil, err
	}

	// Enforce the schema-required attribute even after PATCH: clients
	// must not be able to remove displayName via a future op.
	if dn, _ := next.Properties[PropertyKeyDisplayName].(string); dn == "" {
		return nil, ErrDisplayNameRequired
	}

	if len(next.MemberIDs) > 0 {
		resolved, err := s.repo.ValidateMembers(ctx, tx, providerName, next.MemberIDs)
		if err != nil {
			return nil, err
		}
		if len(resolved) != len(next.MemberIDs) {
			return nil, fmt.Errorf("%w: one or more member references not found in provider tenant", ErrInvalidMember)
		}
	}

	stored, err := s.repo.Replace(ctx, tx, providerName, id, next.Properties, next.MemberIDs)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	beforeJSON, _ := json.Marshal(before.Properties)
	afterJSON, _ := json.Marshal(next.Properties)
	if err := s.audit.Write(ctx, tx, audit.Entry{
		Actor:        scimActor(providerName),
		Operation:    audit.OpUpdate,
		ResourceType: "group",
		ResourceID:   id,
		BeforeValue:  beforeJSON,
		AfterValue:   afterJSON,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	out := FromStored(*stored)
	return &out, nil
}

// memberEntityIDs projects MemberEntity slice to its EntityIDs in order.
func memberEntityIDs(members []MemberEntity) []uuid.UUID {
	if len(members) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.EntityID
	}
	return ids
}

// parseMemberIDs converts SCIM Member.value strings into UUIDs. Every
// value must parse and be unique — duplicates indicate a malformed
// payload and would also blow up the relation unique index later.
func parseMemberIDs(members []Member) ([]uuid.UUID, error) {
	if len(members) == 0 {
		return nil, nil
	}
	seen := make(map[uuid.UUID]struct{}, len(members))
	ids := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		id, err := uuid.Parse(m.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: member value %q is not a UUID", ErrInvalidMember, m.Value)
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("%w: duplicate member %q", ErrInvalidMember, m.Value)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

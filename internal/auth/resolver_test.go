package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/auth"
)

// StubResolverRepo implements auth.ResolverRepository for tests.
type StubResolverRepo struct {
	entityID       uuid.UUID
	entityErr      error
	active         bool
	activeErr      error
	groups         []string
	groupsErr      error
	allowedSystems []uuid.UUID
	systemsErr     error
}

func (s *StubResolverRepo) FindEntityID(_ context.Context, _, _ string) (uuid.UUID, error) {
	return s.entityID, s.entityErr
}

func (s *StubResolverRepo) IsActive(_ context.Context, _ uuid.UUID) (bool, error) {
	return s.active, s.activeErr
}

func (s *StubResolverRepo) Groups(_ context.Context, _ uuid.UUID) ([]string, error) {
	return s.groups, s.groupsErr
}

func (s *StubResolverRepo) AllowedSystems(_ context.Context, entityID uuid.UUID, groupExternalIDs []string) ([]uuid.UUID, error) {
	return s.allowedSystems, s.systemsErr
}

var (
	testEntityID = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	testSystemID = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
)

func TestResolver_Resolve_Success(t *testing.T) {
	repo := &StubResolverRepo{
		entityID:       testEntityID,
		active:         true,
		groups:         []string{"oad:editor"},
		allowedSystems: []uuid.UUID{testSystemID},
	}
	r := auth.NewIdentityResolver(repo)

	id, err := r.Resolve(context.Background(), nil, "keycloak", "user-sub-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.EntityID != testEntityID {
		t.Errorf("EntityID = %v, want %v", id.EntityID, testEntityID)
	}
	if id.Provider != "keycloak" {
		t.Errorf("Provider = %q, want %q", id.Provider, "keycloak")
	}
	if len(id.Groups) != 1 || id.Groups[0] != "oad:editor" {
		t.Errorf("Groups = %v, want [oad:editor]", id.Groups)
	}
	if id.IsPlatformAdmin {
		t.Error("expected IsPlatformAdmin = false for oad:editor member")
	}
	if len(id.AllowedSystems) != 1 {
		t.Errorf("AllowedSystems count = %d, want 1", len(id.AllowedSystems))
	}
}

func TestResolver_Resolve_PlatformAdmin(t *testing.T) {
	repo := &StubResolverRepo{
		entityID: testEntityID,
		active:   true,
		groups:   []string{"oad:admin"},
	}
	r := auth.NewIdentityResolver(repo)
	id, err := r.Resolve(context.Background(), nil, "keycloak", "admin-sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !id.IsPlatformAdmin {
		t.Error("expected IsPlatformAdmin = true for oad:admin member")
	}
}

func TestResolver_Resolve_NotProvisioned(t *testing.T) {
	repo := &StubResolverRepo{entityErr: auth.ErrNotProvisioned}
	r := auth.NewIdentityResolver(repo)
	_, err := r.Resolve(context.Background(), nil, "keycloak", "unknown-sub")
	if !errors.Is(err, auth.ErrNotProvisioned) {
		t.Errorf("expected ErrNotProvisioned, got %v", err)
	}
}

func TestResolver_Resolve_Disabled(t *testing.T) {
	repo := &StubResolverRepo{entityID: testEntityID, active: false}
	r := auth.NewIdentityResolver(repo)
	_, err := r.Resolve(context.Background(), nil, "keycloak", "disabled-sub")
	if !errors.Is(err, auth.ErrDisabled) {
		t.Errorf("expected ErrDisabled, got %v", err)
	}
}

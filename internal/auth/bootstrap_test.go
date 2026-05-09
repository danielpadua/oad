package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/auth"
)

// StubBootstrapRepo tracks calls for test assertions.
type StubBootstrapRepo struct {
	existingLinks    map[string]uuid.UUID // "provider:sub" → entityID
	existingAdmins   map[uuid.UUID]bool   // entityID → has oad:admin relation
	createdUsers     []string             // "provider:sub" entries created
	createdRelations []uuid.UUID          // entityIDs given oad:admin relation
}

func newStubBootstrapRepo() *StubBootstrapRepo {
	return &StubBootstrapRepo{
		existingLinks:  make(map[string]uuid.UUID),
		existingAdmins: make(map[uuid.UUID]bool),
	}
}

func (s *StubBootstrapRepo) FindEntityID(_ context.Context, provider, sub string) (uuid.UUID, error) {
	if id, ok := s.existingLinks[provider+":"+sub]; ok {
		return id, nil
	}
	return uuid.UUID{}, auth.ErrNotProvisioned
}

func (s *StubBootstrapRepo) CreateBootstrapUser(_ context.Context, provider, sub string) (uuid.UUID, error) {
	id := uuid.New()
	s.existingLinks[provider+":"+sub] = id
	s.createdUsers = append(s.createdUsers, provider+":"+sub)
	return id, nil
}

func (s *StubBootstrapRepo) HasAdminRelation(_ context.Context, entityID uuid.UUID) (bool, error) {
	return s.existingAdmins[entityID], nil
}

func (s *StubBootstrapRepo) CreateAdminRelation(_ context.Context, entityID uuid.UUID) error {
	s.existingAdmins[entityID] = true
	s.createdRelations = append(s.createdRelations, entityID)
	return nil
}

func TestBootstrap_CreatesUserAndRelation(t *testing.T) {
	repo := newStubBootstrapRepo()
	b := auth.NewBootstrap(repo)

	err := b.Apply(context.Background(), []auth.BootstrapEntry{
		{Provider: "keycloak", Subject: "admin-sub-001"},
	})
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if len(repo.createdUsers) != 1 {
		t.Errorf("createdUsers = %d, want 1", len(repo.createdUsers))
	}
	if len(repo.createdRelations) != 1 {
		t.Errorf("createdRelations = %d, want 1", len(repo.createdRelations))
	}
}

func TestBootstrap_Idempotent(t *testing.T) {
	repo := newStubBootstrapRepo()
	existingID := uuid.MustParse("cccccccc-0000-0000-0000-000000000001")
	repo.existingLinks["keycloak:admin-sub-001"] = existingID
	repo.existingAdmins[existingID] = true

	b := auth.NewBootstrap(repo)
	if err := b.Apply(context.Background(), []auth.BootstrapEntry{
		{Provider: "keycloak", Subject: "admin-sub-001"},
	}); err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if len(repo.createdUsers) != 0 {
		t.Errorf("expected no new users, got %d", len(repo.createdUsers))
	}
	if len(repo.createdRelations) != 0 {
		t.Errorf("expected no new relations, got %d", len(repo.createdRelations))
	}
}

func TestBootstrap_CreatesRelationForExistingUser(t *testing.T) {
	repo := newStubBootstrapRepo()
	existingID := uuid.MustParse("dddddddd-0000-0000-0000-000000000001")
	repo.existingLinks["keycloak:admin-sub-002"] = existingID

	b := auth.NewBootstrap(repo)
	if err := b.Apply(context.Background(), []auth.BootstrapEntry{
		{Provider: "keycloak", Subject: "admin-sub-002"},
	}); err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if len(repo.createdUsers) != 0 {
		t.Error("expected no new user (entity exists)")
	}
	if len(repo.createdRelations) != 1 {
		t.Errorf("createdRelations = %d, want 1", len(repo.createdRelations))
	}
}

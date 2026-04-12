package auth_test

import (
	"context"
	"testing"

	"github.com/danielpadua/oad/internal/auth"
)

func TestIdentity_HasRole(t *testing.T) {
	id := &auth.Identity{Roles: []string{"admin", "editor"}}

	if !id.HasRole("admin") {
		t.Error("expected HasRole(admin) = true")
	}
	if !id.HasRole("editor") {
		t.Error("expected HasRole(editor) = true")
	}
	if id.HasRole("viewer") {
		t.Error("expected HasRole(viewer) = false")
	}
}

func TestIdentity_HasAnyRole(t *testing.T) {
	id := &auth.Identity{Roles: []string{"viewer"}}

	if !id.HasAnyRole("admin", "viewer") {
		t.Error("expected HasAnyRole(admin, viewer) = true when identity has viewer")
	}
	if id.HasAnyRole("admin", "editor") {
		t.Error("expected HasAnyRole(admin, editor) = false when identity has only viewer")
	}
}

func TestIdentityContext_RoundTrip(t *testing.T) {
	id := &auth.Identity{Subject: "user@example.com", Roles: []string{"admin"}}
	ctx := auth.WithIdentity(context.Background(), id)

	got, ok := auth.IdentityFromContext(ctx)
	if !ok {
		t.Fatal("expected identity in context")
	}
	if got.Subject != "user@example.com" {
		t.Errorf("expected subject user@example.com, got %s", got.Subject)
	}
}

func TestIdentityFromContext_MissingReturnsFalse(t *testing.T) {
	_, ok := auth.IdentityFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for empty context")
	}
}

func TestMustIdentityFromContext_PanicsWhenMissing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when identity is missing")
		}
	}()
	auth.MustIdentityFromContext(context.Background())
}

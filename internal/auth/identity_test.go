package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/auth"
)

func TestIdentity_HasRole(t *testing.T) {
	tests := []struct {
		name            string
		isPlatformAdmin bool
		groups          []string
		role            string
		want            bool
	}{
		{"platform admin has admin", true, nil, "admin", true},
		{"platform admin has editor", true, nil, "editor", true},
		{"platform admin has viewer", true, nil, "viewer", true},
		{"no groups has no admin", false, nil, "admin", false},
		{"oad:editor has editor", false, []string{"oad:editor"}, "editor", true},
		{"oad:editor has viewer (elevated)", false, []string{"oad:editor"}, "viewer", true},
		{"oad:viewer has viewer", false, []string{"oad:viewer"}, "viewer", true},
		{"oad:viewer lacks editor", false, []string{"oad:viewer"}, "editor", false},
		{"oad:viewer lacks admin", false, []string{"oad:viewer"}, "admin", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := &auth.Identity{
				IsPlatformAdmin: tc.isPlatformAdmin,
				Groups:          tc.groups,
			}
			if got := id.HasRole(tc.role); got != tc.want {
				t.Errorf("HasRole(%q) = %v, want %v", tc.role, got, tc.want)
			}
		})
	}
}

func TestIdentity_HasAnyRole(t *testing.T) {
	id := &auth.Identity{Groups: []string{"oad:editor"}}
	if !id.HasAnyRole("admin", "editor") {
		t.Error("expected editor to satisfy HasAnyRole(admin, editor)")
	}
	if id.HasAnyRole("admin") {
		t.Error("expected editor to fail HasAnyRole(admin)")
	}
}

func TestIdentity_IsPlatformAdmin_Flag(t *testing.T) {
	id := &auth.Identity{IsPlatformAdmin: true, EntityID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}
	if !id.HasRole("admin") {
		t.Error("platform admin must satisfy HasRole(admin)")
	}
}

func TestIdentity_ActorString_WithEntityID(t *testing.T) {
	id := &auth.Identity{
		Subject:  "user@example.com",
		EntityID: uuid.MustParse("11111111-0000-0000-0000-000000000001"),
	}
	want := "user:11111111-0000-0000-0000-000000000001"
	if got := id.ActorString(); got != want {
		t.Errorf("ActorString() = %q, want %q", got, want)
	}
}

func TestIdentity_ActorString_NoEntityID(t *testing.T) {
	id := &auth.Identity{Subject: "service-account-cn"}
	if got := id.ActorString(); got != "service-account-cn" {
		t.Errorf("ActorString() = %q, want %q", got, "service-account-cn")
	}
}

func TestIdentityContext_RoundTrip(t *testing.T) {
	original := &auth.Identity{Subject: "test-sub", IsPlatformAdmin: true}
	ctx := auth.WithIdentity(context.Background(), original)
	got, ok := auth.IdentityFromContext(ctx)
	if !ok {
		t.Fatal("IdentityFromContext returned ok=false")
	}
	if got != original {
		t.Error("IdentityFromContext returned different pointer than stored")
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
			t.Error("expected panic, got none")
		}
	}()
	auth.MustIdentityFromContext(context.Background())
}

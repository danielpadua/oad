package users_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/scim/users"
)

func boolPtr(v bool) *bool { return &v }

func TestToProperties(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   users.User
		want map[string]any
	}{
		{
			name: "minimal — defaults active to true",
			in:   users.User{UserName: "alice"},
			want: map[string]any{
				"userName": "alice",
				"active":   true,
			},
		},
		{
			name: "explicit active=false honored",
			in:   users.User{UserName: "alice", Active: boolPtr(false)},
			want: map[string]any{
				"userName": "alice",
				"active":   false,
			},
		},
		{
			name: "displayName preferred over name.formatted",
			in: users.User{
				UserName:    "alice",
				DisplayName: "Alice",
				Name:        &users.Name{Formatted: "Alice Smith"},
			},
			want: map[string]any{
				"userName":    "alice",
				"displayName": "Alice",
				"active":      true,
			},
		},
		{
			name: "name.formatted used when displayName empty",
			in: users.User{
				UserName: "alice",
				Name:     &users.Name{Formatted: "Alice Smith"},
			},
			want: map[string]any{
				"userName":    "alice",
				"displayName": "Alice Smith",
				"active":      true,
			},
		},
		{
			name: "primary email picked",
			in: users.User{
				UserName: "alice",
				Emails: []users.Email{
					{Value: "alice@home.com"},
					{Value: "alice@work.com", Primary: true},
				},
			},
			want: map[string]any{
				"userName": "alice",
				"email":    "alice@work.com",
				"active":   true,
			},
		},
		{
			name: "no primary email — fallback to first",
			in: users.User{
				UserName: "alice",
				Emails:   []users.Email{{Value: "alice@home.com"}},
			},
			want: map[string]any{
				"userName": "alice",
				"email":    "alice@home.com",
				"active":   true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := users.ToProperties(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (got=%v want=%v)", len(got), len(tc.want), got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("got[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func sampleStoredUser(t *testing.T) (users.StoredUser, uuid.UUID, time.Time) {
	t.Helper()
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	return users.StoredUser{
		EntityID: id,
		Properties: map[string]any{
			"userName":    "alice",
			"displayName": "Alice",
			"email":       "alice@oad.dev",
			"active":      true,
		},
		ExternalSubject: "alice-keycloak-id",
		CreatedAt:       now,
		UpdatedAt:       now,
	}, id, now
}

func TestFromStored_IdentityFields(t *testing.T) {
	t.Parallel()
	stored, id, _ := sampleStoredUser(t)
	got := users.FromStored(stored)

	if got.ID != id.String() {
		t.Errorf("ID = %q, want %q", got.ID, id.String())
	}
	if got.ExternalID != "alice-keycloak-id" {
		t.Errorf("ExternalID = %q, want alice-keycloak-id", got.ExternalID)
	}
	if got.UserName != "alice" {
		t.Errorf("UserName = %q, want alice", got.UserName)
	}
	if len(got.Schemas) != 1 || got.Schemas[0] != users.SchemaURN {
		t.Errorf("Schemas = %v, want [User URN]", got.Schemas)
	}
}

func TestFromStored_OptionalProperties(t *testing.T) {
	t.Parallel()
	stored, _, _ := sampleStoredUser(t)
	got := users.FromStored(stored)

	if got.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want Alice", got.DisplayName)
	}
	if len(got.Emails) != 1 || got.Emails[0].Value != "alice@oad.dev" || !got.Emails[0].Primary {
		t.Errorf("Emails = %v, want one primary alice@oad.dev", got.Emails)
	}
	if got.Active == nil || !*got.Active {
		t.Errorf("Active = %v, want &true", got.Active)
	}
}

func TestFromStored_Meta(t *testing.T) {
	t.Parallel()
	stored, id, _ := sampleStoredUser(t)
	got := users.FromStored(stored)

	if got.Meta == nil {
		t.Fatal("Meta is nil")
	}
	if got.Meta.ResourceType != "User" {
		t.Errorf("Meta.ResourceType = %q, want User", got.Meta.ResourceType)
	}
	if got.Meta.Location != "/scim/v2/Users/"+id.String() {
		t.Errorf("Meta.Location = %q, want /scim/v2/Users/<id>", got.Meta.Location)
	}
	if !strings.HasPrefix(got.Meta.Version, `W/"`) {
		t.Errorf("Meta.Version = %q, want weak ETag prefix", got.Meta.Version)
	}
}

func TestETag_DeterministicAndChangesOnUpdate(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	t1 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Second)

	a1 := users.ETag(id, t1)
	a2 := users.ETag(id, t1)
	if a1 != a2 {
		t.Errorf("ETag is not deterministic: %q != %q", a1, a2)
	}
	b := users.ETag(id, t2)
	if a1 == b {
		t.Errorf("ETag did not change on updated_at change: %q == %q", a1, b)
	}
}

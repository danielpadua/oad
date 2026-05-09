package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/handler"
	"github.com/danielpadua/oad/internal/auth"
)

func TestMeHandler_Get(t *testing.T) {
	t.Parallel()

	entityID := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	systemID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	tests := []struct {
		name         string
		identity     *auth.Identity
		wantEntityID string
		wantGroups   []string
		wantSystems  []string
		wantIsAdmin  bool
	}{
		{
			name: "db user with groups and systems",
			identity: &auth.Identity{
				Subject:         "user@example.com",
				Provider:        "keycloak",
				EntityID:        entityID,
				Groups:          []string{"oad:editor"},
				AllowedSystems:  []uuid.UUID{systemID},
				IsPlatformAdmin: false,
				AuthMode:        "jwt",
			},
			wantEntityID: entityID.String(),
			wantGroups:   []string{"oad:editor"},
			wantSystems:  []string{systemID.String()},
			wantIsAdmin:  false,
		},
		{
			name: "platform admin with nil groups",
			identity: &auth.Identity{
				Subject:         "admin@example.com",
				Provider:        "keycloak",
				EntityID:        entityID,
				Groups:          nil,
				IsPlatformAdmin: true,
				AuthMode:        "jwt",
			},
			wantEntityID: entityID.String(),
			wantGroups:   []string{},
			wantSystems:  []string{},
			wantIsAdmin:  true,
		},
		{
			name: "mtls caller with zero entity id",
			identity: &auth.Identity{
				Subject:  "service-cn",
				Provider: "mtls",
				AuthMode: "mtls",
			},
			wantEntityID: "",
			wantGroups:   []string{},
			wantSystems:  []string{},
			wantIsAdmin:  false,
		},
	}

	h := handler.NewMeHandler()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := auth.WithIdentity(t.Context(), tc.identity)
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/me", http.NoBody)
			w := httptest.NewRecorder()

			h.Get(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}

			var got handler.MeResponse
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if got.EntityID != tc.wantEntityID {
				t.Errorf("entity_id = %q, want %q", got.EntityID, tc.wantEntityID)
			}
			if len(got.Groups) != len(tc.wantGroups) {
				t.Errorf("groups count = %d, want %d", len(got.Groups), len(tc.wantGroups))
			}
			if len(got.AllowedSystems) != len(tc.wantSystems) {
				t.Errorf("allowed_systems count = %d, want %d", len(got.AllowedSystems), len(tc.wantSystems))
			}
			if got.IsPlatformAdmin != tc.wantIsAdmin {
				t.Errorf("is_platform_admin = %v, want %v", got.IsPlatformAdmin, tc.wantIsAdmin)
			}
		})
	}
}

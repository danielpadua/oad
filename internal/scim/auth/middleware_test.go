package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielpadua/oad/internal/scim/auth"
)

// nextHandler returns a handler that captures the request context's resolved
// provider for inspection. The captured value is written to *got.
func nextHandler(got *string, called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		if v, ok := auth.ProviderFromContext(r.Context()); ok {
			*got = v
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func newTestRegistry(t *testing.T) *auth.Registry {
	t.Helper()
	r := auth.NewRegistry()
	if err := r.Register("keycloak", "kc-token"); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	return r
}

func TestAuthenticate_AcceptsValidBearer(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	var captured string
	var nextCalled bool
	mw := auth.Authenticate(registry)(nextHandler(&captured, &nextCalled))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users", http.NoBody)
	req.Header.Set("Authorization", "Bearer kc-token")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if !nextCalled {
		t.Errorf("next handler was not called")
	}
	if captured != "keycloak" {
		t.Errorf("captured provider = %q, want keycloak", captured)
	}
}

func TestAuthenticate_RejectsRequests(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)

	cases := []struct {
		name       string
		authHeader string
	}{
		{"no header", ""},
		{"missing bearer prefix", "kc-token"},
		{"basic auth instead of bearer", "Basic kc-token"},
		{"empty bearer token", "Bearer "},
		{"unknown bearer token", "Bearer not-the-real-one"},
		{"case-mismatched token", "Bearer KC-TOKEN"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var captured string
			var nextCalled bool
			mw := auth.Authenticate(registry)(nextHandler(&captured, &nextCalled))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users", http.NoBody)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()
			mw.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rr.Code)
			}
			if nextCalled {
				t.Errorf("next handler was called for rejected request")
			}
			if got, want := rr.Header().Get("Content-Type"), "application/scim+json"; got != want {
				t.Errorf("Content-Type = %q, want %q", got, want)
			}

			var body map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			schemas, _ := body["schemas"].([]any)
			if len(schemas) != 1 || schemas[0] != "urn:ietf:params:scim:api:messages:2.0:Error" {
				t.Errorf("schemas = %v, want SCIM Error URN", schemas)
			}
			if body["status"] != "401" {
				t.Errorf("status field = %v, want \"401\"", body["status"])
			}
		})
	}
}

func TestProviderFromContext_Empty(t *testing.T) {
	t.Parallel()

	if v, ok := auth.ProviderFromContext(t.Context()); ok || v != "" {
		t.Errorf("empty context returned (%q, %v); want (\"\", false)", v, ok)
	}
}

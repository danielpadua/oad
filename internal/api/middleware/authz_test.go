package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/middleware"
	"github.com/danielpadua/oad/internal/auth"
)

func withIdentity(r *http.Request, id *auth.Identity) *http.Request {
	return r.WithContext(auth.WithIdentity(r.Context(), id))
}

func TestRequireRole_Allows(t *testing.T) {
	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "u", IsPlatformAdmin: true})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRole_Rejects(t *testing.T) {
	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "u", Groups: []string{"oad:viewer"}})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_NoIdentity(t *testing.T) {
	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAnyRole_Allows(t *testing.T) {
	handler := middleware.RequireAnyRole("admin", "editor")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "u", Groups: []string{"oad:editor"}})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireAnyRole_Rejects(t *testing.T) {
	handler := middleware.RequireAnyRole("admin", "editor")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "u", Groups: []string{"oad:viewer"}})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireSystemScope_PlatformAdminAllowed(t *testing.T) {
	// Platform admins must also supply ActiveSystemID (set via X-OAD-System-Id header
	// by Authentication middleware). No special bypass anymore.
	sysID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	handler := middleware.RequireSystemScope("systemID")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "admin", IsPlatformAdmin: true, ActiveSystemID: &sysID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for platform admin with ActiveSystemID, got %d", rec.Code)
	}
}

func TestRequireSystemScope_PlatformAdminNoSystemID(t *testing.T) {
	// Platform admins without ActiveSystemID are also rejected.
	handler := middleware.RequireSystemScope("systemID")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "admin", IsPlatformAdmin: true})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for platform admin without ActiveSystemID, got %d", rec.Code)
	}
}

func TestRequireSystemScope_MatchingSystem(t *testing.T) {
	handler := middleware.RequireSystemScope("systemID")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	sysID := uuid.MustParse("00000000-0000-0000-0000-000000000123")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "svc", ActiveSystemID: &sysID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for identity with ActiveSystemID, got %d", rec.Code)
	}
}

func TestRequireSystemScope_MissingActiveSystemID(t *testing.T) {
	// Identity without ActiveSystemID is rejected with 400.
	handler := middleware.RequireSystemScope("systemID")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "svc"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing ActiveSystemID, got %d", rec.Code)
	}
}

func TestAuthentication_SystemIdHeader_ValidSystem(t *testing.T) {
	systemID := uuid.MustParse("eeeeeeee-0000-0000-0000-000000000001")
	// We test the header parsing logic through RequireSystemScope + a fake identity
	// (Authentication middleware test requires a real cache which is integration scope).
	// Instead, test the resulting ActiveSystemID behavior via auth.WithIdentity.
	identity := &auth.Identity{
		IsPlatformAdmin: false,
		AllowedSystems:  []uuid.UUID{systemID},
		ActiveSystemID:  &systemID,
	}

	handler := middleware.RequireSystemScope("system_id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, identity)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireSystemScope_MissingHeader(t *testing.T) {
	handler := middleware.RequireSystemScope("system_id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	identity := &auth.Identity{IsPlatformAdmin: false} // no ActiveSystemID

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, identity)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRequireSystemScope_PlatformAdminWithActiveSystem(t *testing.T) {
	systemID := uuid.MustParse("ffffffff-0000-0000-0000-000000000001")
	handler := middleware.RequireSystemScope("system_id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	identity := &auth.Identity{IsPlatformAdmin: true, ActiveSystemID: &systemID}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req = withIdentity(req, identity)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for admin with active system, got %d", rec.Code)
	}
}

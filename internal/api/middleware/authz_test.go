package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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
	r := chi.NewRouter()
	r.Route("/systems/{systemID}", func(r chi.Router) {
		r.Use(middleware.RequireSystemScope("systemID"))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/systems/some-uuid", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "admin", IsPlatformAdmin: true})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for platform admin, got %d", rec.Code)
	}
}

func TestRequireSystemScope_MatchingSystem(t *testing.T) {
	r := chi.NewRouter()
	r.Route("/systems/{systemID}", func(r chi.Router) {
		r.Use(middleware.RequireSystemScope("systemID"))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	sysID := uuid.MustParse("00000000-0000-0000-0000-000000000123")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/systems/"+sysID.String(), http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "svc", ActiveSystemID: &sysID})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for matching system, got %d", rec.Code)
	}
}

func TestRequireSystemScope_CrossSystemDenied(t *testing.T) {
	r := chi.NewRouter()
	r.Route("/systems/{systemID}", func(r chi.Router) {
		r.Use(middleware.RequireSystemScope("systemID"))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	sysID := uuid.MustParse("00000000-0000-0000-0000-000000000123")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/systems/00000000-0000-0000-0000-000000000999", http.NoBody)
	req = withIdentity(req, &auth.Identity{Subject: "svc", ActiveSystemID: &sysID})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-system access, got %d", rec.Code)
	}
}

// Package api wires together all HTTP handlers, middleware, and routes.
// The router is the single composition root for the HTTP layer.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/api/handler"
	"github.com/danielpadua/oad/internal/api/middleware"
	"github.com/danielpadua/oad/internal/config"
)

// Dependencies holds all external dependencies injected into the HTTP layer.
// Using an explicit struct instead of individual parameters makes the
// constructor signature stable as the application grows.
type Dependencies struct {
	DB     *pgxpool.Pool
	Config *config.Config
	Logger *slog.Logger
}

// NewRouter constructs the Chi router with all middleware and routes registered.
// Middleware order matters: Recovery must be outermost so it catches panics from
// all other middleware; CorrelationID must precede RequestLogger so the ID is
// available when the log entry is written.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	// --- Global middleware (applied to every request) ---
	r.Use(middleware.Recovery)
	r.Use(middleware.CorrelationID)
	r.Use(middleware.RequestLogger)
	r.Use(chimw.Compress(5))

	// --- Handlers ---
	healthHandler := handler.NewHealthHandler(deps.DB)

	// --- Routes ---
	// Operational endpoints are intentionally outside /api/v1 to keep
	// them accessible to load balancers and Prometheus without auth.
	r.Get("/health", healthHandler.Get)

	// /api/v1 will host all domain endpoints (Phases 2–6).
	r.Route("/api/v1", func(r chi.Router) {
		// Phase 1: auth + system-scope middleware will be mounted here.
		// Phase 2+: domain routes will be registered here.
	})

	return r
}

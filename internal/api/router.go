// Package api wires together all HTTP handlers, middleware, and routes.
// The router is the single composition root for the HTTP layer.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/danielpadua/oad/internal/api/handler"
	"github.com/danielpadua/oad/internal/api/middleware"
	"github.com/danielpadua/oad/internal/auth"
	"github.com/danielpadua/oad/internal/config"
)

// Dependencies holds all external dependencies injected into the HTTP layer.
// Using an explicit struct instead of individual parameters makes the
// constructor signature stable as the application grows.
type Dependencies struct {
	DB              *pgxpool.Pool
	Config          *config.Config
	Logger          *slog.Logger
	MetricsRegistry prometheus.Registerer   // nil defaults to prometheus.DefaultRegisterer
	JWTAuth         *auth.JWTAuthenticator  // nil when AUTH_MODE is "mtls"
	MTLSAuth        *auth.MTLSAuthenticator // nil when AUTH_MODE is "jwt"
}

// NewRouter constructs the Chi router with all middleware and routes registered.
// Middleware order matters: Recovery must be outermost so it catches panics from
// all other middleware; CorrelationID must precede RequestLogger so the ID is
// available when the log entry is written.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	// Default to the global Prometheus registry when none is injected.
	reg := deps.MetricsRegistry
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	// --- Global middleware (applied to every request) ---
	r.Use(middleware.Recovery)
	r.Use(middleware.CorrelationID)
	metrics := middleware.NewMetricsMiddleware(reg)
	r.Use(metrics.Handler)
	r.Use(middleware.RequestLogger)
	r.Use(chimw.Compress(5))

	// --- Handlers ---
	healthHandler := handler.NewHealthHandler(deps.DB)

	// --- Routes ---
	// Operational endpoints are intentionally outside /api/v1 to keep
	// them accessible to load balancers and Prometheus without auth.
	r.Get("/health", healthHandler.Get)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// /api/v1 hosts all domain endpoints (Phases 2–6).
	// Authentication is required for every request in this group.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Authentication(deps.JWTAuth, deps.MTLSAuth, deps.Config.Auth.Mode))
		// Phase 2+: domain routes and per-group authz middleware registered here.
	})

	return r
}

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
// Using an explicit struct instead of individual parameters keeps the
// constructor signature stable as the application grows.
type Dependencies struct {
	DB              *pgxpool.Pool
	Config          *config.Config
	Logger          *slog.Logger
	MetricsRegistry prometheus.Registerer   // nil defaults to prometheus.DefaultRegisterer
	JWTAuth         *auth.JWTAuthenticator  // nil when AUTH_MODE is "mtls"
	MTLSAuth        *auth.MTLSAuthenticator // nil when AUTH_MODE is "jwt"

	// Phase 2 handlers — Schema Registry
	EntityTypeHandler    *handler.EntityTypeHandler
	SystemHandler        *handler.SystemHandler
	OverlaySchemaHandler *handler.OverlaySchemaHandler

	// Phase 3 handlers — Entity & Relation Management
	EntityHandler   *handler.EntityHandler
	RelationHandler *handler.RelationHandler
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

	// /api/v1 — all domain endpoints require authentication.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Authentication(deps.JWTAuth, deps.MTLSAuth, deps.Config.Auth.Mode))

		// ── Phase 2 — Schema Registry ────────────────────────────────────
		// All schema registry endpoints require the "admin" role.

		// Entity type definitions: global schema registry.
		r.Route("/entity-types", func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/", deps.EntityTypeHandler.List)
			r.Post("/", deps.EntityTypeHandler.Create)
			r.Get("/{type_id}", deps.EntityTypeHandler.GetByID)
			r.Put("/{type_id}", deps.EntityTypeHandler.Update)
			r.Delete("/{type_id}", deps.EntityTypeHandler.Delete)
		})

		// Systems and their overlay schemas.
		r.Route("/systems", func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/", deps.SystemHandler.List)
			r.Post("/", deps.SystemHandler.Create)

			r.Route("/{system_id}", func(r chi.Router) {
				r.Get("/", deps.SystemHandler.GetByID)
				r.Patch("/", deps.SystemHandler.Patch)

				// Overlay schemas nested under their owning system.
				r.Route("/overlay-schemas", func(r chi.Router) {
					r.Get("/", deps.OverlaySchemaHandler.List)
					r.Post("/", deps.OverlaySchemaHandler.Create)
					r.Get("/{schema_id}", deps.OverlaySchemaHandler.GetByID)
					r.Put("/{schema_id}", deps.OverlaySchemaHandler.Update)
					r.Delete("/{schema_id}", deps.OverlaySchemaHandler.Delete)
				})
			})
		})

		// ── Phase 3 — Entity & Relation Management ────────────────────────

		// Entities: typed nodes in the authorization graph.
		// Literal sub-paths (/lookup, /bulk) are registered before /{entity_id}
		// so chi's radix tree resolves them as literals, not path parameters.
		r.Route("/entities", func(r chi.Router) {
			// Exact lookup by type + external_id (no role restriction — any
			// authenticated caller may look up entities, e.g., a PDP).
			r.Get("/lookup", deps.EntityHandler.Lookup)

			// Write operations require at least editor role.
			r.With(middleware.RequireAnyRole("admin", "editor")).Post("/bulk", deps.EntityHandler.BulkCreate)
			r.With(middleware.RequireAnyRole("admin", "editor")).Post("/", deps.EntityHandler.Create)

			// List is open to all authenticated roles.
			r.Get("/", deps.EntityHandler.List)

			r.Route("/{entity_id}", func(r chi.Router) {
				r.Get("/", deps.EntityHandler.GetByID)
				r.With(middleware.RequireAnyRole("admin", "editor")).Patch("/", deps.EntityHandler.Update)
				r.With(middleware.RequireAnyRole("admin", "editor")).Delete("/", deps.EntityHandler.Delete)

				// Relations of this entity (FR-REL-005).
				r.Get("/relations", deps.RelationHandler.ListByEntity)
			})
		})

		// Relations: directed edges between entities.
		r.Route("/relations", func(r chi.Router) {
			r.With(middleware.RequireAnyRole("admin", "editor")).Post("/", deps.RelationHandler.Create)

			r.Route("/{relation_id}", func(r chi.Router) {
				r.Get("/", deps.RelationHandler.GetByID)
				r.With(middleware.RequireAnyRole("admin", "editor")).Delete("/", deps.RelationHandler.Delete)
			})
		})
	})

	return r
}

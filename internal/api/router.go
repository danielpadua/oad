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
	scimauth "github.com/danielpadua/oad/internal/scim/auth"
	scimhandler "github.com/danielpadua/oad/internal/scim/handler"
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
	IdentityCache   *auth.IdentityCache     // LRU+TTL cache wrapping the identity resolver (Phase 9.C)

	// Phase 2 handlers — Schema Registry
	EntityTypeHandler    *handler.EntityTypeHandler
	SystemHandler        *handler.SystemHandler
	OverlaySchemaHandler *handler.OverlaySchemaHandler

	// Phase 3 handlers — Entity & Relation Management
	EntityHandler   *handler.EntityHandler
	RelationHandler *handler.RelationHandler

	// Phase 4 handlers — Overlay System
	OverlayHandler *handler.OverlayHandler

	// Phase 5 handlers — Retrieval API
	RetrievalHandler *handler.RetrievalHandler

	// Phase 6 handlers — Webhooks
	WebhookHandler *handler.WebhookHandler

	// Phase 7 handlers — Dashboard
	StatsHandler *handler.StatsHandler

	// Frontend bootstrap — OIDC configuration
	ConfigHandler *handler.ConfigHandler

	// Phase 9.B — SCIM tenant token registry (nil disables /scim/v2 mount).
	SCIMRegistry *scimauth.Registry

	// Phase 9.B.3 — SCIM /Users handler (nil disables /Users routes; discovery
	// endpoints are still served when SCIMRegistry is non-nil).
	SCIMUsersHandler *scimhandler.UsersHandler

	// Phase 9.B.4 — SCIM /Groups handler (nil disables /Groups routes).
	SCIMGroupsHandler *scimhandler.GroupsHandler

	// Phase 9.C — /me endpoint
	MeHandler *handler.MeHandler

	// Phase 9.D — Admin UI for Users
	UsersHandler *handler.UsersHandler

	// Embedded Management UI — SPA catch-all (nil disables the UI).
	WebUIHandler http.Handler
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
	r.Get("/config.json", deps.ConfigHandler.Get)

	// /scim/v2 — SCIM 2.0 ingest surface (Phase 9.B). Discovery endpoints
	// are unauthenticated; resource endpoints (Users/Groups, Phase 9.B.3+)
	// will use the SCIM tenant token registry for bearer auth.
	if deps.SCIMRegistry != nil {
		scimhandler.Mount(r, deps.SCIMRegistry, scimhandler.Handlers{
			Users:  deps.SCIMUsersHandler,
			Groups: deps.SCIMGroupsHandler,
		})
	}

	// /api/v1 — all domain endpoints require authentication.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Authentication(deps.JWTAuth, deps.MTLSAuth, deps.IdentityCache, deps.Config.Auth.Mode))

		// ── Phase 9.C — Caller identity introspection ─────────────────────
		r.Get("/me", deps.MeHandler.Get)

		// ── Phase 2 — Schema Registry ────────────────────────────────────
		// All schema registry endpoints require the "admin" role.

		// Entity type definitions: global schema registry.
		// Reads are open to all authenticated roles — system-admin and editor users
		// need the type list to create overlay schemas and entities.
		// Writes are restricted to platform admins.
		r.Route("/entity-types", func(r chi.Router) {
			r.Get("/", deps.EntityTypeHandler.List)
			r.With(middleware.RequirePlatformAdmin).Post("/", deps.EntityTypeHandler.Create)
			r.Get("/{type_id}", deps.EntityTypeHandler.GetByID)
			r.With(middleware.RequirePlatformAdmin).Put("/{type_id}", deps.EntityTypeHandler.Update)
			r.With(middleware.RequirePlatformAdmin).Delete("/{type_id}", deps.EntityTypeHandler.Delete)
		})

		// Systems and their overlay schemas.
		// Platform admins have full access.
		// system-admin users can list/view/patch (description only) and manage overlay schemas
		// for systems assigned via has_role_in.
		// editor and viewer users can list and view their assigned systems (read-only).
		r.Route("/systems", func(r chi.Router) {
			// List: all roles that have system assignments can see their systems.
			// Service filters by AllowedSystems for non-platform-admins.
			r.With(middleware.RequireAnyRole("admin", "system-admin", "editor", "viewer")).
				Get("/", deps.SystemHandler.List)

			// Create: registering a new system is a platform-level action.
			r.With(middleware.RequirePlatformAdmin).Post("/", deps.SystemHandler.Create)

			r.Route("/{system_id}", func(r chi.Router) {
				// Detail: any role may view, but only their allowed systems.
				r.With(
					middleware.RequireAnyRole("admin", "system-admin", "editor", "viewer"),
					middleware.RequirePathSystemInScope("system_id"),
				).Get("/", deps.SystemHandler.GetByID)

				// Patch: system-admin and above; scoped to allowed systems.
				r.With(
					middleware.RequireAnyRole("admin", "system-admin"),
					middleware.RequirePathSystemInScope("system_id"),
				).Patch("/", deps.SystemHandler.Patch)

				// Overlay schemas: system-admin and above, scoped to allowed systems.
				r.Route("/overlay-schemas", func(r chi.Router) {
					r.Use(middleware.RequireAnyRole("admin", "system-admin"))
					r.Use(middleware.RequirePathSystemInScope("system_id"))
					r.Get("/", deps.OverlaySchemaHandler.List)
					r.Post("/", deps.OverlaySchemaHandler.Create)
					r.Get("/{schema_id}", deps.OverlaySchemaHandler.GetByID)
					r.Put("/{schema_id}", deps.OverlaySchemaHandler.Update)
					r.Delete("/{schema_id}", deps.OverlaySchemaHandler.Delete)
				})

				// Webhook subscriptions: platform admin only, scoped via RequireSystemScope.
				r.Route("/webhooks", func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Use(middleware.RequireSystemScope("system_id"))
					r.Get("/", deps.WebhookHandler.List)
					r.Post("/", deps.WebhookHandler.Create)

					r.Route("/{webhook_id}", func(r chi.Router) {
						r.Get("/", deps.WebhookHandler.GetByID)
						r.Patch("/", deps.WebhookHandler.Update)
						r.Delete("/", deps.WebhookHandler.Delete)
					})
				})
			})
		})

		// ── Phase 9.D — Admin UI for Users ───────────────────────────────
		r.Route("/users", func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/", deps.UsersHandler.List)
		})

		// ── Dashboard stats ──────────────────────────────────────────────
		// Aggregate counters for the management UI. Open to any authenticated
		// caller; counts are global regardless of the caller's system scope.
		r.Get("/stats", deps.StatsHandler.Get)

		// ── Phase 5 — Retrieval API (top-level) ──────────────────────────
		// These endpoints are open to all authenticated callers (PDPs, control planes).
		r.Get("/changelog", deps.RetrievalHandler.Changelog)
		r.Get("/export", deps.RetrievalHandler.Export)

		// ── Phase 3 — Entity & Relation Management ────────────────────────

		// Entities: typed nodes in the authorization graph.
		// Literal sub-paths (/lookup, /search, /bulk) are registered before /{entity_id}
		// so chi's radix tree resolves them as literals, not path parameters.
		r.Route("/entities", func(r chi.Router) {
			// Phase 5 lookup: type + external_id with optional system_id for merged view.
			// Supersedes the Phase 3 simple lookup with full retrieval logging (FR-RET-001).
			r.Get("/lookup", deps.RetrievalHandler.Lookup)

			// Phase 5 property filter: JSONB containment queries via GIN index (FR-RET-002).
			r.Get("/search", deps.RetrievalHandler.Filter)

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

				// ── Phase 4 — Overlay System ──────────────────────────────────
				// Property overlays are scoped to the caller's system (FR-OVL-008).
				// Reads are open to all authenticated roles; writes require editor+.
				r.Route("/overlays", func(r chi.Router) {
					r.Get("/", deps.OverlayHandler.List)
					r.With(middleware.RequireAnyRole("admin", "editor")).Post("/", deps.OverlayHandler.Create)

					r.Route("/{overlay_id}", func(r chi.Router) {
						r.Get("/", deps.OverlayHandler.GetByID)
						r.With(middleware.RequireAnyRole("admin", "editor")).Put("/", deps.OverlayHandler.Update)
						r.With(middleware.RequireAnyRole("admin", "editor")).Delete("/", deps.OverlayHandler.Delete)
					})
				})
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

	// Management UI catch-all — must be last so it never shadows API routes.
	// Unknown paths fall through to index.html; React Router handles them client-side.
	if deps.WebUIHandler != nil {
		r.Handle("/*", deps.WebUIHandler)
	}

	return r
}

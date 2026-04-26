package handler

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/db"
)

// StatsHandler serves platform-wide aggregate metrics for the dashboard.
// Counts are global (not scoped to the caller's system) — the endpoint runs
// inside an RLS-unscoped transaction so callers see totals across all tenants.
type StatsHandler struct {
	db *pgxpool.Pool
}

// NewStatsHandler constructs a StatsHandler.
func NewStatsHandler(pool *pgxpool.Pool) *StatsHandler {
	return &StatsHandler{db: pool}
}

type statsResponse struct {
	TotalEntities      int64 `json:"total_entities"`
	ActiveSystems      int64 `json:"active_systems"`
	SubscribedWebhooks int64 `json:"subscribed_webhooks"`
}

// Get handles GET /api/v1/stats.
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	var out statsResponse

	// Use WithSystemScope with an empty systemID so RLS evaluates in
	// "admin mode" (the policy allows access when app.current_system_id = '').
	err := db.WithSystemScope(r.Context(), h.db, "", func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM entity`).Scan(&out.TotalEntities); err != nil {
			return fmt.Errorf("counting entities: %w", err)
		}
		if err := tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM system WHERE active = true`).Scan(&out.ActiveSystems); err != nil {
			return fmt.Errorf("counting active systems: %w", err)
		}
		if err := tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM webhook_subscription WHERE active = true`).Scan(&out.SubscribedWebhooks); err != nil {
			return fmt.Errorf("counting webhook subscriptions: %w", err)
		}
		return nil
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}

	response.JSON(w, http.StatusOK, out)
}

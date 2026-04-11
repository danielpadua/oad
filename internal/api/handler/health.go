package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/api/response"
)

// HealthHandler handles the /health endpoint.
// Reports application and database connectivity status (NFR-OPS-001).
type HealthHandler struct {
	db *pgxpool.Pool
}

func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{db: db}
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// Get responds with the current health status.
// Returns 200 OK when healthy, 503 Service Unavailable when the database
// is unreachable. Load balancer health checks rely on this endpoint.
func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dbStatus := "ok"
	if err := h.db.Ping(ctx); err != nil {
		dbStatus = "unreachable"
	}

	status := "ok"
	httpStatus := http.StatusOK

	if dbStatus != "ok" {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	response.JSON(w, httpStatus, healthResponse{
		Status:   status,
		Database: dbStatus,
	})
}

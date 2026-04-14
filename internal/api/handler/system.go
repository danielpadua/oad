package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/system"
)

// systemService is the interface the handler depends on.
type systemService interface {
	Create(ctx context.Context, req system.CreateRequest) (*system.System, error)
	GetByID(ctx context.Context, id uuid.UUID) (*system.System, error)
	List(ctx context.Context) ([]*system.System, error)
	Patch(ctx context.Context, id uuid.UUID, req system.PatchRequest) (*system.System, error)
}

// SystemHandler handles HTTP requests for system endpoints.
type SystemHandler struct {
	svc systemService
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(svc systemService) *SystemHandler {
	return &SystemHandler{svc: svc}
}

// List handles GET /api/v1/systems
func (h *SystemHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

// Create handles POST /api/v1/systems
func (h *SystemHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req system.CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	sys, err := h.svc.Create(r.Context(), req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusCreated, sys)
}

// GetByID handles GET /api/v1/systems/{system_id}
func (h *SystemHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}

	sys, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, sys)
}

// Patch handles PATCH /api/v1/systems/{system_id}
// Supports partial updates of name, description, and active (FR-SYS-002, FR-SYS-003).
func (h *SystemHandler) Patch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}

	var req system.PatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	sys, err := h.svc.Patch(r.Context(), id, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, sys)
}

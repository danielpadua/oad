package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/overlay"
)

// overlayService is the interface the handler depends on,
// enabling substitution in tests without the full service graph.
type overlayService interface {
	Create(ctx context.Context, entityID uuid.UUID, req overlay.CreateRequest) (*overlay.PropertyOverlay, error)
	GetByID(ctx context.Context, entityID, overlayID uuid.UUID) (*overlay.PropertyOverlay, error)
	ListByEntity(ctx context.Context, entityID uuid.UUID, params overlay.ListParams) (*overlay.ListResult, error)
	Update(ctx context.Context, entityID, overlayID uuid.UUID, req overlay.UpdateRequest) (*overlay.PropertyOverlay, error)
	Delete(ctx context.Context, entityID, overlayID uuid.UUID) error
}

// OverlayHandler handles HTTP requests for property overlay endpoints.
type OverlayHandler struct {
	svc overlayService
}

// NewOverlayHandler creates a new property overlay handler.
func NewOverlayHandler(svc overlayService) *OverlayHandler {
	return &OverlayHandler{svc: svc}
}

// List handles GET /api/v1/entities/{entity_id}/overlays
// Accepts optional ?limit={n}&offset={n} query parameters.
// Admins see all overlays for the entity; system-scoped callers see only their own.
func (h *OverlayHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}
	limit, offset := parsePagination(r)

	result, err := h.svc.ListByEntity(r.Context(), entityID, overlay.ListParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Create handles POST /api/v1/entities/{entity_id}/overlays (FR-OVL-001)
// The system_id is derived from the caller's token — not accepted from the request body.
func (h *OverlayHandler) Create(w http.ResponseWriter, r *http.Request) {
	entityID, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}

	var req overlay.CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	o, err := h.svc.Create(r.Context(), entityID, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusCreated, o)
}

// GetByID handles GET /api/v1/entities/{entity_id}/overlays/{overlay_id}
func (h *OverlayHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	entityID, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}
	overlayID, ok := pathUUID(w, r, "overlay_id")
	if !ok {
		return
	}

	o, err := h.svc.GetByID(r.Context(), entityID, overlayID)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, o)
}

// Update handles PUT /api/v1/entities/{entity_id}/overlays/{overlay_id} (FR-OVL-001)
// Replaces the overlay properties; system scope is enforced by the service layer.
func (h *OverlayHandler) Update(w http.ResponseWriter, r *http.Request) {
	entityID, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}
	overlayID, ok := pathUUID(w, r, "overlay_id")
	if !ok {
		return
	}

	var req overlay.UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	o, err := h.svc.Update(r.Context(), entityID, overlayID, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, o)
}

// Delete handles DELETE /api/v1/entities/{entity_id}/overlays/{overlay_id}
// Only the owning system may delete its overlay (FR-OVL-008).
func (h *OverlayHandler) Delete(w http.ResponseWriter, r *http.Request) {
	entityID, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}
	overlayID, ok := pathUUID(w, r, "overlay_id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), entityID, overlayID); err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.NoContent(w)
}

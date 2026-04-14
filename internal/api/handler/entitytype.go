package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/entitytype"
)

// entityTypeService is the interface the handler depends on,
// enabling substitution in tests without the full service graph.
type entityTypeService interface {
	Create(ctx context.Context, req entitytype.CreateRequest) (*entitytype.EntityTypeDefinition, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entitytype.EntityTypeDefinition, error)
	List(ctx context.Context) ([]*entitytype.EntityTypeDefinition, error)
	Update(ctx context.Context, id uuid.UUID, req entitytype.UpdateRequest) (*entitytype.EntityTypeDefinition, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// EntityTypeHandler handles HTTP requests for entity type definition endpoints.
type EntityTypeHandler struct {
	svc entityTypeService
}

// NewEntityTypeHandler creates a new entity type handler.
func NewEntityTypeHandler(svc entityTypeService) *EntityTypeHandler {
	return &EntityTypeHandler{svc: svc}
}

// List handles GET /api/v1/entity-types
func (h *EntityTypeHandler) List(w http.ResponseWriter, r *http.Request) {
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

// Create handles POST /api/v1/entity-types
func (h *EntityTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req entitytype.CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	etd, err := h.svc.Create(r.Context(), req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusCreated, etd)
}

// GetByID handles GET /api/v1/entity-types/{type_id}
func (h *EntityTypeHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "type_id")
	if !ok {
		return
	}

	etd, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, etd)
}

// Update handles PUT /api/v1/entity-types/{type_id}
func (h *EntityTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "type_id")
	if !ok {
		return
	}

	var req entitytype.UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	etd, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, etd)
}

// Delete handles DELETE /api/v1/entity-types/{type_id}
func (h *EntityTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "type_id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.NoContent(w)
}

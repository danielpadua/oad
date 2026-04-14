package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/entity"
)

// entityService is the interface the handler depends on,
// enabling substitution in tests without the full service graph.
type entityService interface {
	Create(ctx context.Context, req entity.CreateRequest) (*entity.Entity, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Entity, error)
	GetByTypeAndExternalID(ctx context.Context, typeName, externalID string) (*entity.Entity, error)
	List(ctx context.Context, params entity.ListParams) (*entity.ListResult, error)
	Update(ctx context.Context, id uuid.UUID, req entity.UpdateRequest) (*entity.Entity, error)
	Delete(ctx context.Context, id uuid.UUID) error
	BulkCreate(ctx context.Context, req entity.BulkCreateRequest) (*entity.BulkCreateResult, error)
}

// EntityHandler handles HTTP requests for entity endpoints.
type EntityHandler struct {
	svc entityService
}

// NewEntityHandler creates a new entity handler.
func NewEntityHandler(svc entityService) *EntityHandler {
	return &EntityHandler{svc: svc}
}

// List handles GET /api/v1/entities
// Accepts optional ?type={type_name}&limit={n}&offset={n} query parameters.
func (h *EntityHandler) List(w http.ResponseWriter, r *http.Request) {
	typeName := r.URL.Query().Get("type")
	limit, offset := parsePagination(r)

	result, err := h.svc.List(r.Context(), entity.ListParams{
		TypeName: typeName,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Lookup handles GET /api/v1/entities/lookup
// Requires ?type={type_name}&external_id={external_id} query parameters (FR-ENT-004).
func (h *EntityHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	typeName := r.URL.Query().Get("type")
	externalID := r.URL.Query().Get("external_id")

	if typeName == "" || externalID == "" {
		response.HandleError(r.Context(), w,
			badRequestErr("type and external_id query parameters are required"))
		return
	}

	e, err := h.svc.GetByTypeAndExternalID(r.Context(), typeName, externalID)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, e)
}

// Create handles POST /api/v1/entities
func (h *EntityHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req entity.CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	e, err := h.svc.Create(r.Context(), req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusCreated, e)
}

// BulkCreate handles POST /api/v1/entities/bulk (FR-ENT-007)
func (h *EntityHandler) BulkCreate(w http.ResponseWriter, r *http.Request) {
	var req entity.BulkCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.BulkCreate(r.Context(), req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// GetByID handles GET /api/v1/entities/{entity_id}
func (h *EntityHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}

	e, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, e)
}

// Update handles PATCH /api/v1/entities/{entity_id} (FR-ENT-005)
func (h *EntityHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}

	var req entity.UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	e, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, e)
}

// Delete handles DELETE /api/v1/entities/{entity_id} (FR-ENT-006)
func (h *EntityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.NoContent(w)
}

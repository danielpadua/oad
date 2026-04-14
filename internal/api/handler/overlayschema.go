package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/overlayschema"
)

// overlaySchemaService is the interface the handler depends on.
type overlaySchemaService interface {
	Create(ctx context.Context, systemID uuid.UUID, req overlayschema.CreateRequest) (*overlayschema.SystemOverlaySchema, error)
	GetByID(ctx context.Context, systemID, schemaID uuid.UUID) (*overlayschema.SystemOverlaySchema, error)
	ListBySystem(ctx context.Context, systemID uuid.UUID) ([]*overlayschema.SystemOverlaySchema, error)
	Update(ctx context.Context, systemID, schemaID uuid.UUID, req overlayschema.UpdateRequest) (*overlayschema.SystemOverlaySchema, error)
	Delete(ctx context.Context, systemID, schemaID uuid.UUID) error
}

// OverlaySchemaHandler handles HTTP requests for system overlay schema endpoints.
type OverlaySchemaHandler struct {
	svc overlaySchemaService
}

// NewOverlaySchemaHandler creates a new overlay schema handler.
func NewOverlaySchemaHandler(svc overlaySchemaService) *OverlaySchemaHandler {
	return &OverlaySchemaHandler{svc: svc}
}

// List handles GET /api/v1/systems/{system_id}/overlay-schemas
func (h *OverlaySchemaHandler) List(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}

	items, err := h.svc.ListBySystem(r.Context(), systemID)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

// Create handles POST /api/v1/systems/{system_id}/overlay-schemas
func (h *OverlaySchemaHandler) Create(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}

	var req overlayschema.CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	schema, err := h.svc.Create(r.Context(), systemID, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusCreated, schema)
}

// GetByID handles GET /api/v1/systems/{system_id}/overlay-schemas/{schema_id}
func (h *OverlaySchemaHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}
	schemaID, ok := pathUUID(w, r, "schema_id")
	if !ok {
		return
	}

	schema, err := h.svc.GetByID(r.Context(), systemID, schemaID)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, schema)
}

// Update handles PUT /api/v1/systems/{system_id}/overlay-schemas/{schema_id}
func (h *OverlaySchemaHandler) Update(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}
	schemaID, ok := pathUUID(w, r, "schema_id")
	if !ok {
		return
	}

	var req overlayschema.UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	schema, err := h.svc.Update(r.Context(), systemID, schemaID, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, schema)
}

// Delete handles DELETE /api/v1/systems/{system_id}/overlay-schemas/{schema_id}
func (h *OverlaySchemaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}
	schemaID, ok := pathUUID(w, r, "schema_id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), systemID, schemaID); err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.NoContent(w)
}

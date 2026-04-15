package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/relation"
)

// relationService is the interface the handler depends on.
type relationService interface {
	Create(ctx context.Context, req relation.CreateRequest) (*relation.Relation, error)
	GetByID(ctx context.Context, id uuid.UUID) (*relation.Relation, error)
	ListByEntity(ctx context.Context, entityID uuid.UUID, params relation.ListParams) (*relation.ListResult, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// RelationHandler handles HTTP requests for relation endpoints.
type RelationHandler struct {
	svc    relationService
	retLog retrievalLogger // optional; when non-nil, retrieval events are logged (FR-AUD-002)
}

// NewRelationHandler creates a new relation handler.
// retLog may be nil; when nil, retrieval logging for relation queries is skipped.
func NewRelationHandler(svc relationService, retLog retrievalLogger) *RelationHandler {
	return &RelationHandler{svc: svc, retLog: retLog}
}

// Create handles POST /api/v1/relations (FR-REL-001)
func (h *RelationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req relation.CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	rel, err := h.svc.Create(r.Context(), req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusCreated, rel)
}

// GetByID handles GET /api/v1/relations/{relation_id}
func (h *RelationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "relation_id")
	if !ok {
		return
	}

	rel, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, rel)
}

// ListByEntity handles GET /api/v1/entities/{entity_id}/relations (FR-REL-005)
// Accepts optional ?relation_type={rt}&system_id={uuid}&limit={n}&offset={n}.
func (h *RelationHandler) ListByEntity(w http.ResponseWriter, r *http.Request) {
	entityID, ok := pathUUID(w, r, "entity_id")
	if !ok {
		return
	}

	limit, offset := parsePagination(r)

	systemID, ok := queryUUID(w, r, "system_id")
	if !ok {
		return
	}

	params := relation.ListParams{
		RelationType: r.URL.Query().Get("relation_type"),
		SystemID:     systemID,
		Limit:        limit,
		Offset:       offset,
	}
	result, err := h.svc.ListByEntity(r.Context(), entityID, params)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}

	// Log the retrieval event (FR-AUD-002).
	if h.retLog != nil {
		ids := make([]string, len(result.Items))
		for i, rel := range result.Items {
			ids[i] = rel.ID.String()
		}
		queryParams, _ := json.Marshal(map[string]any{
			"entity_id":     entityID,
			"relation_type": params.RelationType,
			"system_id":     params.SystemID,
			"limit":         params.Limit,
			"offset":        params.Offset,
		})
		returnedRefs, _ := json.Marshal(ids)
		h.retLog.LogRetrieval(r.Context(), queryParams, returnedRefs)
	}

	response.JSON(w, http.StatusOK, result)
}

// Delete handles DELETE /api/v1/relations/{relation_id} (FR-REL-004)
func (h *RelationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "relation_id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.NoContent(w)
}

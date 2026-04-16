package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/webhook"
)

// webhookService is the interface the handler depends on,
// enabling substitution in tests without the full service graph.
type webhookService interface {
	Create(ctx context.Context, systemID uuid.UUID, req webhook.CreateRequest) (*webhook.Subscription, error)
	GetByID(ctx context.Context, systemID, subscriptionID uuid.UUID) (*webhook.Subscription, error)
	List(ctx context.Context, systemID uuid.UUID, params webhook.ListParams) (*webhook.ListResult, error)
	Update(ctx context.Context, systemID, subscriptionID uuid.UUID, req webhook.UpdateRequest) (*webhook.Subscription, error)
	Delete(ctx context.Context, systemID, subscriptionID uuid.UUID) error
}

// WebhookHandler handles HTTP requests for webhook subscription endpoints.
type WebhookHandler struct {
	svc webhookService
}

// NewWebhookHandler creates a new webhook subscription handler.
func NewWebhookHandler(svc webhookService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

// List handles GET /api/v1/systems/{system_id}/webhooks
// Accepts optional ?limit={n}&offset={n} query parameters.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}
	limit, offset := parsePagination(r)

	result, err := h.svc.List(r.Context(), systemID, webhook.ListParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Create handles POST /api/v1/systems/{system_id}/webhooks (FR-WHK-001).
// The secret is accepted in the request body and stored as the HMAC-SHA256 signing key;
// it is not returned in the response.
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}

	var req webhook.CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	sub, err := h.svc.Create(r.Context(), systemID, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusCreated, sub)
}

// GetByID handles GET /api/v1/systems/{system_id}/webhooks/{webhook_id}
func (h *WebhookHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}
	webhookID, ok := pathUUID(w, r, "webhook_id")
	if !ok {
		return
	}

	sub, err := h.svc.GetByID(r.Context(), systemID, webhookID)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, sub)
}

// Update handles PATCH /api/v1/systems/{system_id}/webhooks/{webhook_id} (FR-WHK-003).
// Supports partial updates: callback_url and/or active flag.
func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}
	webhookID, ok := pathUUID(w, r, "webhook_id")
	if !ok {
		return
	}

	var req webhook.UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	sub, err := h.svc.Update(r.Context(), systemID, webhookID, req)
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, sub)
}

// Delete handles DELETE /api/v1/systems/{system_id}/webhooks/{webhook_id} (FR-WHK-003).
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathUUID(w, r, "system_id")
	if !ok {
		return
	}
	webhookID, ok := pathUUID(w, r, "webhook_id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), systemID, webhookID); err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.NoContent(w)
}

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	scimauth "github.com/danielpadua/oad/internal/scim/auth"
	"github.com/danielpadua/oad/internal/scim/groups"
	"github.com/danielpadua/oad/internal/scim/response"
)

// GroupsService is the contract the GroupsHandler depends on. Declared in
// the handler package so handler tests can supply a stub without pulling
// in the database-backed service.
type GroupsService interface {
	Create(ctx context.Context, providerName string, g groups.Group) (*groups.Group, error)
	GetByEntityID(ctx context.Context, providerName string, id uuid.UUID) (*groups.Group, error)
	List(ctx context.Context, providerName, filterStr string, offset, limit int) (*groups.ListResult, error)
	Replace(ctx context.Context, providerName string, id uuid.UUID, g groups.Group) (*groups.Group, error)
	Delete(ctx context.Context, providerName string, id uuid.UUID) error
}

// groupsListResponse is the SCIM ListResponse shape (RFC 7644 §3.4.2)
// for the /Groups endpoint.
type groupsListResponse struct {
	Schemas      []string        `json:"schemas"`
	TotalResults int             `json:"totalResults"`
	ItemsPerPage int             `json:"itemsPerPage"`
	StartIndex   int             `json:"startIndex"`
	Resources    []*groups.Group `json:"Resources"`
}

// GroupsHandler serves /scim/v2/Groups endpoints. It is configured at
// composition time with a GroupsService and remains stateless thereafter.
type GroupsHandler struct {
	svc GroupsService
}

// NewGroupsHandler returns a GroupsHandler bound to the given service.
func NewGroupsHandler(svc GroupsService) *GroupsHandler {
	return &GroupsHandler{svc: svc}
}

// Create handles POST /scim/v2/Groups.
func (h *GroupsHandler) Create(w http.ResponseWriter, r *http.Request) {
	provider, ok := scimauth.ProviderFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "no SCIM provider in context")
		return
	}

	var in groups.Group
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidSyntax", "malformed JSON: "+err.Error())
		return
	}

	created, err := h.svc.Create(r.Context(), provider, in)
	if err != nil {
		writeGroupError(w, err)
		return
	}

	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	response.WriteJSON(w, http.StatusCreated, created)
}

// GetByID handles GET /scim/v2/Groups/{id}.
func (h *GroupsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	provider, ok := scimauth.ProviderFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "no SCIM provider in context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidValue", "id must be a UUID")
		return
	}

	got, err := h.svc.GetByEntityID(r.Context(), provider, id)
	if err != nil {
		writeGroupError(w, err)
		return
	}

	w.Header().Set("ETag", got.Meta.Version)
	response.WriteJSON(w, http.StatusOK, got)
}

// List handles GET /scim/v2/Groups with optional ?filter, ?startIndex, ?count.
// startIndex is 1-based per RFC 7644 §3.4.2.4; count defaults to 50 and
// is clamped to 200 to match ServiceProviderConfig.filter.maxResults.
func (h *GroupsHandler) List(w http.ResponseWriter, r *http.Request) {
	provider, ok := scimauth.ProviderFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "no SCIM provider in context")
		return
	}

	q := r.URL.Query()
	filterStr := q.Get("filter")
	startIndex := max(parseQueryInt(q.Get("startIndex"), 1), 1)
	count := min(max(parseQueryInt(q.Get("count"), defaultListCount), 0), maxListCount)

	result, err := h.svc.List(r.Context(), provider, filterStr, startIndex-1, count)
	if err != nil {
		writeGroupError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, groupsListResponse{
		Schemas:      []string{listResponseURN},
		TotalResults: result.Total,
		ItemsPerPage: len(result.Groups),
		StartIndex:   startIndex,
		Resources:    result.Groups,
	})
}

// Replace handles PUT /scim/v2/Groups/{id} (RFC 7644 §3.5.1). The full
// resource representation is replaced; entity_id, the (provider,
// external_subject) link, and created_at are preserved. Membership is
// reconciled against the new members[] payload — relations not in the
// new set are dropped, and new ones are inserted.
func (h *GroupsHandler) Replace(w http.ResponseWriter, r *http.Request) {
	provider, ok := scimauth.ProviderFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "no SCIM provider in context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidValue", "id must be a UUID")
		return
	}

	var in groups.Group
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidSyntax", "malformed JSON: "+err.Error())
		return
	}

	updated, err := h.svc.Replace(r.Context(), provider, id, in)
	if err != nil {
		writeGroupError(w, err)
		return
	}

	w.Header().Set("ETag", updated.Meta.Version)
	response.WriteJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /scim/v2/Groups/{id}.
func (h *GroupsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	provider, ok := scimauth.ProviderFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "no SCIM provider in context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidValue", "id must be a UUID")
		return
	}

	if err := h.svc.Delete(r.Context(), provider, id); err != nil {
		writeGroupError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeGroupError maps domain errors to SCIM HTTP responses (RFC 7644 §3.12).
func writeGroupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, groups.ErrNotFound):
		response.WriteError(w, http.StatusNotFound, "", "group not found")
	case errors.Is(err, groups.ErrAlreadyExists):
		response.WriteError(w, http.StatusConflict, "uniqueness", "externalId already provisioned for this provider")
	case errors.Is(err, groups.ErrDisplayNameRequired):
		response.WriteError(w, http.StatusBadRequest, "invalidValue", "displayName is required")
	case errors.Is(err, groups.ErrExternalIDRequired):
		response.WriteError(w, http.StatusBadRequest, "invalidValue", "externalId is required on create")
	case errors.Is(err, groups.ErrInvalidMember):
		response.WriteError(w, http.StatusBadRequest, "invalidValue", err.Error())
	case errors.Is(err, groups.ErrInvalidFilter):
		response.WriteError(w, http.StatusBadRequest, "invalidFilter", err.Error())
	case errors.Is(err, groups.ErrBuiltinGroup):
		response.WriteError(w, http.StatusForbidden, "mutability", "built-in group cannot be modified via SCIM")
	default:
		response.WriteError(w, http.StatusInternalServerError, "", err.Error())
	}
}

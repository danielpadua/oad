package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	scimauth "github.com/danielpadua/oad/internal/scim/auth"
	"github.com/danielpadua/oad/internal/scim/response"
	"github.com/danielpadua/oad/internal/scim/users"
)

const (
	defaultListCount = 50
	maxListCount     = 200

	listResponseURN = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
)

// UsersService is the contract the UsersHandler depends on. Declared in the
// handler package so handler tests can supply a stub without pulling in
// the database-backed service.
type UsersService interface {
	Create(ctx context.Context, providerName string, u users.User) (*users.User, error)
	GetByEntityID(ctx context.Context, providerName string, id uuid.UUID) (*users.User, error)
	List(ctx context.Context, providerName, filterStr string, offset, limit int) (*users.ListResult, error)
	Replace(ctx context.Context, providerName string, id uuid.UUID, u users.User) (*users.User, error)
	Patch(ctx context.Context, providerName string, id uuid.UUID, ops []users.PatchOp) (*users.User, error)
	Delete(ctx context.Context, providerName string, id uuid.UUID) error
}

// usersListResponse is the SCIM ListResponse shape (RFC 7644 §3.4.2) for
// the /Users endpoint. Resources is typed as []*users.User so the JSON
// shape matches the Schema-declared SCIM User without going through any.
type usersListResponse struct {
	Schemas      []string      `json:"schemas"`
	TotalResults int           `json:"totalResults"`
	ItemsPerPage int           `json:"itemsPerPage"`
	StartIndex   int           `json:"startIndex"`
	Resources    []*users.User `json:"Resources"`
}

// UsersHandler serves /scim/v2/Users endpoints. It is configured at
// composition time with a UsersService and remains stateless thereafter.
type UsersHandler struct {
	svc UsersService
}

// NewUsersHandler returns a UsersHandler bound to the given service.
func NewUsersHandler(svc UsersService) *UsersHandler {
	return &UsersHandler{svc: svc}
}

// Create handles POST /scim/v2/Users.
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	provider, ok := scimauth.ProviderFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "no SCIM provider in context")
		return
	}

	var in users.User
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidSyntax", "malformed JSON: "+err.Error())
		return
	}

	created, err := h.svc.Create(r.Context(), provider, in)
	if err != nil {
		writeUserError(w, err)
		return
	}

	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	response.WriteJSON(w, http.StatusCreated, created)
}

// GetByID handles GET /scim/v2/Users/{id}.
func (h *UsersHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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
		writeUserError(w, err)
		return
	}

	w.Header().Set("ETag", got.Meta.Version)
	response.WriteJSON(w, http.StatusOK, got)
}

// List handles GET /scim/v2/Users with optional ?filter, ?startIndex, ?count.
// startIndex is 1-based per RFC 7644 §3.4.2.4; count defaults to 50 and is
// clamped to 200 to match ServiceProviderConfig.filter.maxResults.
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
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
		writeUserError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, usersListResponse{
		Schemas:      []string{listResponseURN},
		TotalResults: result.Total,
		ItemsPerPage: len(result.Users),
		StartIndex:   startIndex,
		Resources:    result.Users,
	})
}

// Replace handles PUT /scim/v2/Users/{id} (RFC 7644 §3.5.1). The full
// resource representation is replaced; entity_id, the (provider,
// external_subject) link, and created_at are preserved.
func (h *UsersHandler) Replace(w http.ResponseWriter, r *http.Request) {
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

	var in users.User
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidSyntax", "malformed JSON: "+err.Error())
		return
	}

	updated, err := h.svc.Replace(r.Context(), provider, id, in)
	if err != nil {
		writeUserError(w, err)
		return
	}

	w.Header().Set("ETag", updated.Meta.Version)
	response.WriteJSON(w, http.StatusOK, updated)
}

// Patch handles PATCH /scim/v2/Users/{id} (RFC 7644 §3.5.2). Each entry
// in Operations[] is applied in order; the first failure aborts the
// batch and the transaction is rolled back. The supported subset is
// documented on users.ApplyPatch.
func (h *UsersHandler) Patch(w http.ResponseWriter, r *http.Request) {
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

	var req users.PatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalidSyntax", "malformed JSON: "+err.Error())
		return
	}

	updated, err := h.svc.Patch(r.Context(), provider, id, req.Operations)
	if err != nil {
		writeUserError(w, err)
		return
	}

	w.Header().Set("ETag", updated.Meta.Version)
	response.WriteJSON(w, http.StatusOK, updated)
}

// parseQueryInt parses an int from a query parameter; missing or invalid
// values return def.
func parseQueryInt(raw string, def int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// Delete handles DELETE /scim/v2/Users/{id}.
func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
		writeUserError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeUserError maps domain errors to SCIM HTTP responses (RFC 7644 §3.12).
func writeUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrNotFound):
		response.WriteError(w, http.StatusNotFound, "", "user not found")
	case errors.Is(err, users.ErrAlreadyExists):
		response.WriteError(w, http.StatusConflict, "uniqueness", "externalId already provisioned for this provider")
	case errors.Is(err, users.ErrUserNameRequired):
		response.WriteError(w, http.StatusBadRequest, "invalidValue", "userName is required")
	case errors.Is(err, users.ErrExternalIDRequired):
		response.WriteError(w, http.StatusBadRequest, "invalidValue", "externalId is required on create")
	case errors.Is(err, users.ErrInvalidFilter):
		response.WriteError(w, http.StatusBadRequest, "invalidFilter", err.Error())
	case errors.Is(err, users.ErrPatchNoTarget):
		response.WriteError(w, http.StatusBadRequest, "noTarget", err.Error())
	case errors.Is(err, users.ErrPatchInvalidValue):
		response.WriteError(w, http.StatusBadRequest, "invalidValue", err.Error())
	default:
		response.WriteError(w, http.StatusInternalServerError, "", err.Error())
	}
}

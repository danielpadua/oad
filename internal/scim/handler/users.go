package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	scimauth "github.com/danielpadua/oad/internal/scim/auth"
	"github.com/danielpadua/oad/internal/scim/response"
	"github.com/danielpadua/oad/internal/scim/users"
)

// UsersService is the contract the UsersHandler depends on. Declared in the
// handler package so handler tests can supply a stub without pulling in
// the database-backed service.
type UsersService interface {
	Create(ctx context.Context, providerName string, u users.User) (*users.User, error)
	GetByEntityID(ctx context.Context, providerName string, id uuid.UUID) (*users.User, error)
	Delete(ctx context.Context, providerName string, id uuid.UUID) error
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
	default:
		response.WriteError(w, http.StatusInternalServerError, "", err.Error())
	}
}

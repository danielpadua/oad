package handler

import (
	"context"
	"net/http"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/useradmin"
)

type userService interface {
	List(ctx context.Context, params useradmin.ListParams) (*useradmin.ListResult, error)
}

// UsersHandler handles HTTP requests for admin user endpoints.
type UsersHandler struct {
	svc userService
}

// NewUsersHandler creates a new user handler.
func NewUsersHandler(svc userService) *UsersHandler {
	return &UsersHandler{svc: svc}
}

// List handles GET /api/v1/users
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	result, err := h.svc.List(r.Context(), useradmin.ListParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

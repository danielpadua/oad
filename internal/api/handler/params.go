package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/apierr"
)

// decodeJSON deserializes the JSON request body into dst.
// Writes a 400 error and returns false on failure; the caller must return.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		response.Error(w, apierr.BadRequest("invalid JSON: "+err.Error()))
		return false
	}
	return true
}

// pathUUID parses a URL parameter as a UUID.
// Writes a 400 error and returns false on parse failure; the caller must return.
func pathUUID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		response.Error(w, apierr.BadRequest("invalid "+param+": must be a valid UUID"))
		return uuid.UUID{}, false
	}
	return id, true
}

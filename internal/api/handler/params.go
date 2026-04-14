package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// parsePagination extracts limit and offset from query parameters.
// Defaults: limit=25, offset=0. Limit is capped at 100.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 25
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

// queryUUID parses a named query parameter as a UUID pointer.
// Returns (nil, true) when the parameter is absent (valid, means "no filter").
// Returns (nil, false) when the parameter is present but not a valid UUID; a 400
// error is written and the caller must return.
func queryUUID(w http.ResponseWriter, r *http.Request, param string) (*uuid.UUID, bool) {
	val := r.URL.Query().Get(param)
	if val == "" {
		return nil, true
	}
	id, err := uuid.Parse(val)
	if err != nil {
		response.Error(w, apierr.BadRequest("invalid "+param+": must be a valid UUID"))
		return nil, false
	}
	return &id, true
}

// badRequestErr returns an *apierr.APIError for use in handler error paths.
func badRequestErr(msg string) *apierr.APIError {
	return apierr.BadRequest(msg)
}

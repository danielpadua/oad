// Package response provides helpers for writing consistent JSON HTTP responses.
package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/danielpadua/oad/internal/apierr"
)

// JSON writes a JSON-encoded body with the given status code.
// A nil body writes the status code with no body.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if body == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// Error writes a structured APIError response.
func Error(w http.ResponseWriter, err *apierr.APIError) {
	JSON(w, err.HTTPStatus, err)
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

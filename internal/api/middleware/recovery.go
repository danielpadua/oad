package middleware

import (
	"log/slog"
	"net/http"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/apierr"
)

// Recovery catches panics from downstream handlers, logs them with the
// correlation ID, and returns a 500 Internal Server Error to the client.
// This prevents unhandled panics from crashing the server process.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "panic recovered",
					"panic", rec,
					"correlation_id", GetCorrelationID(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
				)
				response.Error(w, apierr.Internal("an unexpected error occurred"))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

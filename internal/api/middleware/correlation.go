package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const correlationIDKey contextKey = "correlationID"

// HeaderCorrelationID is the HTTP header name for the correlation ID.
const HeaderCorrelationID = "X-Correlation-ID"

// CorrelationID extracts the correlation ID from the request header or generates
// a new UUID if absent. The ID is stored in the request context and echoed in
// the response header for end-to-end request tracing (NFR-OPS-002).
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderCorrelationID)
		if id == "" {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), correlationIDKey, id)
		w.Header().Set(HeaderCorrelationID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCorrelationID retrieves the correlation ID from the context.
// Returns an empty string if not set.
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// CorrelationIDExtractor is a logging.ContextExtractor that injects the
// correlation_id attribute into every log record automatically.
func CorrelationIDExtractor(ctx context.Context) []slog.Attr {
	if id := GetCorrelationID(ctx); id != "" {
		return []slog.Attr{slog.String("correlation_id", id)}
	}
	return nil
}

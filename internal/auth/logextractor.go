package auth

import (
	"context"
	"log/slog"
)

// IdentityExtractor is a logging.ContextExtractor that injects the
// authenticated caller's subject and system_id into every log record.
func IdentityExtractor(ctx context.Context) []slog.Attr {
	id, ok := IdentityFromContext(ctx)
	if !ok {
		return nil
	}
	attrs := []slog.Attr{slog.String("actor", id.Subject)}
	if id.SystemID != "" {
		attrs = append(attrs, slog.String("system_id", id.SystemID))
	}
	return attrs
}

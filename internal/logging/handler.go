// Package logging provides a context-aware slog handler that automatically
// injects request-scoped attributes (correlation ID, actor, system scope)
// into every log record without requiring callers to pass them explicitly.
package logging

import (
	"context"
	"log/slog"
)

// ContextExtractor pulls attributes from a context for automatic injection
// into log records. Each extractor is called on every log entry; it should
// return nil when its value is absent from the context.
type ContextExtractor func(ctx context.Context) []slog.Attr

// ContextHandler wraps an inner slog.Handler and enriches every record with
// attributes extracted from the request context. Register extractors at
// startup; the handler calls them on each Handle invocation.
type ContextHandler struct {
	inner      slog.Handler
	extractors []ContextExtractor
}

// NewContextHandler creates a handler that delegates to inner after injecting
// context-derived attributes via the provided extractors.
func NewContextHandler(inner slog.Handler, extractors ...ContextExtractor) *ContextHandler {
	return &ContextHandler{
		inner:      inner,
		extractors: extractors,
	}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, rec slog.Record) error {
	for _, extract := range h.extractors {
		if attrs := extract(ctx); len(attrs) > 0 {
			rec.AddAttrs(attrs...)
		}
	}
	return h.inner.Handle(ctx, rec)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{
		inner:      h.inner.WithAttrs(attrs),
		extractors: h.extractors,
	}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{
		inner:      h.inner.WithGroup(name),
		extractors: h.extractors,
	}
}

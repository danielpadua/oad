package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/retrieval"
)

// retrievalSvc is the interface the RetrievalHandler depends on.
type retrievalSvc interface {
	Lookup(ctx context.Context, params retrieval.LookupParams) (*retrieval.MergedEntityView, error)
	Filter(ctx context.Context, params retrieval.FilterParams) (*retrieval.FilterResult, error)
	ListChangelog(ctx context.Context, params retrieval.ChangelogParams) (*retrieval.ChangelogResult, error)
	Export(ctx context.Context, params retrieval.ExportParams) (*retrieval.ExportResult, error)
}

// retrievalLogger is satisfied by *retrieval.Service and allows other handlers
// (e.g. RelationHandler) to write retrieval log entries (FR-AUD-002).
type retrievalLogger interface {
	LogRetrieval(ctx context.Context, queryParams, returnedRefs json.RawMessage)
}

// RetrievalHandler handles the PDP-facing retrieval API (Phase 5):
// entity lookup with merged view, property filter, changelog, and bulk export.
type RetrievalHandler struct {
	svc retrievalSvc
}

// NewRetrievalHandler creates a new retrieval handler.
func NewRetrievalHandler(svc retrievalSvc) *RetrievalHandler {
	return &RetrievalHandler{svc: svc}
}

// Lookup handles GET /api/v1/entities/lookup
// Required: ?type=, ?external_id=
// Optional: ?system_id= — when present triggers the merged property + relation view
// (FR-RET-001, FR-OVL-006, FR-OVL-007).
func (h *RetrievalHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	typeName := r.URL.Query().Get("type")
	externalID := r.URL.Query().Get("external_id")

	if typeName == "" || externalID == "" {
		response.HandleError(r.Context(), w,
			badRequestErr("type and external_id query parameters are required"))
		return
	}

	systemID, ok := queryUUID(w, r, "system_id")
	if !ok {
		return
	}

	view, err := h.svc.Lookup(r.Context(), retrieval.LookupParams{
		TypeName:   typeName,
		ExternalID: externalID,
		SystemID:   systemID,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, view)
}

// Filter handles GET /api/v1/entities/search
// Required: ?filter=<json-object> — JSONB containment predicate (e.g. {"department":"ops"})
// Optional: ?type=, ?limit=, ?offset= (FR-RET-002).
func (h *RetrievalHandler) Filter(w http.ResponseWriter, r *http.Request) {
	filterStr := r.URL.Query().Get("filter")
	if filterStr == "" {
		response.HandleError(r.Context(), w,
			badRequestErr("filter query parameter is required"))
		return
	}
	// Ensure the filter is a valid JSON object before forwarding to the DB.
	var filterCheck map[string]json.RawMessage
	if err := json.Unmarshal([]byte(filterStr), &filterCheck); err != nil {
		response.HandleError(r.Context(), w,
			badRequestErr("filter must be a valid JSON object (e.g. {\"department\":\"ops\"})"))
		return
	}

	limit, offset := parsePagination(r)

	result, err := h.svc.Filter(r.Context(), retrieval.FilterParams{
		TypeName: r.URL.Query().Get("type"),
		Filter:   json.RawMessage(filterStr),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Changelog handles GET /api/v1/changelog
// Required: ?since=<RFC3339> — lower bound on audit_log.timestamp
// Optional: ?system_id=, ?limit=, ?offset= (FR-RET-003).
func (h *RetrievalHandler) Changelog(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	if sinceStr == "" {
		response.HandleError(r.Context(), w,
			badRequestErr("since query parameter is required (RFC3339 format, e.g. 2026-01-01T00:00:00Z)"))
		return
	}
	since, err := time.Parse(time.RFC3339, sinceStr)
	if err != nil {
		response.HandleError(r.Context(), w,
			badRequestErr("since must be a valid RFC3339 timestamp (e.g. 2026-01-01T00:00:00Z)"))
		return
	}

	systemID, ok := queryUUID(w, r, "system_id")
	if !ok {
		return
	}

	limit, offset := parsePagination(r)

	result, err := h.svc.ListChangelog(r.Context(), retrieval.ChangelogParams{
		Since:    since,
		SystemID: systemID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Export handles GET /api/v1/export
// Optional: ?type=, ?limit= (default 100, max 500), ?offset= (FR-RET-004).
func (h *RetrievalHandler) Export(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	result, err := h.svc.Export(r.Context(), retrieval.ExportParams{
		TypeName: r.URL.Query().Get("type"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		response.HandleError(r.Context(), w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

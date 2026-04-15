package retrieval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/auth"
)

// Service orchestrates PDP-facing retrieval operations.
// Every successful retrieval writes a best-effort entry to retrieval_log (FR-AUD-002).
type Service struct {
	pool *pgxpool.Pool
	repo Repository
}

// NewService creates a new retrieval service.
func NewService(pool *pgxpool.Pool, repo Repository) *Service {
	return &Service{pool: pool, repo: repo}
}

// Lookup returns a merged entity view (FR-RET-001, FR-OVL-006, FR-OVL-007).
// When params.SystemID is set, property overlays are merged in and system-scoped
// relations are included. System-scoped callers may only request their own system.
func (s *Service) Lookup(ctx context.Context, params LookupParams) (*MergedEntityView, error) {
	if params.TypeName == "" {
		return nil, apierr.BadRequest("type is required")
	}
	if params.ExternalID == "" {
		return nil, apierr.BadRequest("external_id is required")
	}
	if apiErr := s.assertSystemAccess(ctx, params.SystemID); apiErr != nil {
		return nil, apiErr
	}

	view, err := s.repo.LookupMerged(ctx, s.pool, params)
	if err != nil {
		return nil, toAPIError(err)
	}

	queryParams, _ := json.Marshal(map[string]any{
		"type":        params.TypeName,
		"external_id": params.ExternalID,
		"system_id":   params.SystemID,
	})
	s.logRetrieval(ctx, params.SystemID, queryParams, uuidRefsJSON(view.ID))

	return view, nil
}

// Filter returns entities whose properties satisfy a JSONB containment filter (FR-RET-002).
func (s *Service) Filter(ctx context.Context, params FilterParams) (*FilterResult, error) {
	if len(params.Filter) == 0 {
		return nil, apierr.BadRequest("filter is required")
	}
	if params.Limit <= 0 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	items, total, err := s.repo.FilterByProperties(ctx, s.pool, params)
	if err != nil {
		return nil, fmt.Errorf("filtering entities: %w", err)
	}

	ids := make([]uuid.UUID, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	queryParams, _ := json.Marshal(map[string]any{
		"type":   params.TypeName,
		"filter": params.Filter,
		"limit":  params.Limit,
		"offset": params.Offset,
	})
	s.logRetrieval(ctx, nil, queryParams, uuidRefsJSON(ids...))

	return &FilterResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// ListChangelog returns paginated audit_log entries since a given timestamp (FR-RET-003).
// System-scoped callers are automatically restricted to their own system's events.
func (s *Service) ListChangelog(ctx context.Context, params ChangelogParams) (*ChangelogResult, error) {
	// Enforce system isolation: system-scoped callers see only their own system's events.
	identity := auth.MustIdentityFromContext(ctx)
	if identity.SystemID != "" {
		callerSysID, err := uuid.Parse(identity.SystemID)
		if err != nil {
			return nil, apierr.Internal("invalid system_id in token")
		}
		// If the caller explicitly requested a different system, reject it.
		if params.SystemID != nil && *params.SystemID != callerSysID {
			return nil, apierr.Forbidden("system-scoped callers may only query their own system's changelog")
		}
		params.SystemID = &callerSysID
	}

	if params.Limit <= 0 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	items, total, err := s.repo.ListChangelog(ctx, s.pool, params)
	if err != nil {
		return nil, fmt.Errorf("listing changelog: %w", err)
	}

	queryParams, _ := json.Marshal(map[string]any{
		"since":     params.Since,
		"system_id": params.SystemID,
		"limit":     params.Limit,
		"offset":    params.Offset,
	})
	s.logRetrieval(ctx, params.SystemID, queryParams, json.RawMessage(`[]`))

	return &ChangelogResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// Export returns a deterministically-ordered page of entities with their relations (FR-RET-004).
// The system context for relation scoping is derived from the caller's auth identity.
func (s *Service) Export(ctx context.Context, params ExportParams) (*ExportResult, error) {
	// Derive system context for relation scoping from auth identity.
	identity := auth.MustIdentityFromContext(ctx)
	if identity.SystemID != "" && params.SystemID == nil {
		sysID, err := uuid.Parse(identity.SystemID)
		if err != nil {
			return nil, apierr.Internal("invalid system_id in token")
		}
		params.SystemID = &sysID
	}

	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > 500 {
		params.Limit = 500
	}

	items, total, err := s.repo.ExportEntities(ctx, s.pool, params)
	if err != nil {
		return nil, fmt.Errorf("exporting entities: %w", err)
	}

	ids := make([]uuid.UUID, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	queryParams, _ := json.Marshal(map[string]any{
		"type":   params.TypeName,
		"limit":  params.Limit,
		"offset": params.Offset,
	})
	s.logRetrieval(ctx, params.SystemID, queryParams, uuidRefsJSON(ids...))

	return &ExportResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// LogRetrieval writes a retrieval log entry on behalf of external callers
// (e.g., the relation handler). Errors are logged but not surfaced (best-effort).
func (s *Service) LogRetrieval(ctx context.Context, queryParams, returnedRefs json.RawMessage) {
	s.logRetrieval(ctx, nil, queryParams, returnedRefs)
}

// assertSystemAccess ensures system-scoped callers may only request data for their
// own system. Platform admins (empty SystemID in token) bypass this check.
func (s *Service) assertSystemAccess(ctx context.Context, requestedSystemID *uuid.UUID) *apierr.APIError {
	identity := auth.MustIdentityFromContext(ctx)
	if identity.SystemID == "" || requestedSystemID == nil {
		return nil // admin, or no system context requested
	}
	callerSysID, err := uuid.Parse(identity.SystemID)
	if err != nil {
		return apierr.Internal("invalid system_id in token")
	}
	if callerSysID != *requestedSystemID {
		return apierr.Forbidden("system-scoped callers may only request their own system's data")
	}
	return nil
}

// logRetrieval writes a retrieval_log row. Errors are logged but not propagated
// to the caller — a retrieval log failure must never block a valid PDP query.
func (s *Service) logRetrieval(ctx context.Context, systemID *uuid.UUID, queryParams, returnedRefs json.RawMessage) {
	identity := auth.MustIdentityFromContext(ctx)
	entry := LogEntry{
		CallerIdentity: identity.Subject,
		QueryParams:    queryParams,
		ReturnedRefs:   returnedRefs,
	}
	if systemID != nil {
		sid := systemID.String()
		entry.SystemID = &sid
	} else if identity.SystemID != "" {
		entry.SystemID = &identity.SystemID
	}
	if err := s.repo.WriteLog(ctx, s.pool, entry); err != nil {
		slog.WarnContext(ctx, "failed to write retrieval log", "error", err)
	}
}

// uuidRefsJSON serializes a list of UUIDs as a JSON array for retrieval_log.
func uuidRefsJSON(ids ...uuid.UUID) json.RawMessage {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	b, _ := json.Marshal(strs)
	return b
}

// toAPIError maps domain errors to *apierr.APIError.
func toAPIError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *apierr.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	if errors.Is(err, ErrEntityNotFound) {
		return apierr.NotFound("entity")
	}
	return err
}

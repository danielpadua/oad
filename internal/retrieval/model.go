// Package retrieval implements the PDP-facing retrieval API: entity lookup with a merged
// property view, property filter queries, changelog, and bulk export (Phase 5).
// All retrieval operations write a best-effort entry to retrieval_log (FR-AUD-002).
package retrieval

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MergedEntityView is the AuthZen-compatible entity representation returned by the
// Retrieval API. When a system_id is requested, Properties contain the global properties
// merged with the system's overlay (FR-OVL-006). Relations include global edges plus
// system-scoped edges for the requested system (FR-OVL-007).
type MergedEntityView struct {
	ID         uuid.UUID       `json:"id"`
	TypeID     uuid.UUID       `json:"type_id"`
	Type       string          `json:"type"`
	ExternalID string          `json:"external_id"`
	Properties json.RawMessage `json:"properties"` // global || overlay when system is requested
	Relations  []RelationRef   `json:"relations"`
	SystemID   *uuid.UUID      `json:"system_id,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// RelationRef is a compact relation reference embedded in a MergedEntityView or ExportItem.
type RelationRef struct {
	ID             uuid.UUID  `json:"id"`
	RelationType   string     `json:"relation_type"`
	TargetEntityID uuid.UUID  `json:"target_entity_id"`
	SystemID       *uuid.UUID `json:"system_id,omitempty"`
}

// LookupParams controls the entity lookup endpoint (FR-RET-001, FR-OVL-006, FR-OVL-007).
type LookupParams struct {
	TypeName   string
	ExternalID string
	SystemID   *uuid.UUID // when set, triggers the merged view
}

// FilterParams controls the property filter endpoint (FR-RET-002).
// Filter must be a JSON object suitable for PostgreSQL's @> (containment) operator.
type FilterParams struct {
	TypeName string          // optional type pre-filter
	Filter   json.RawMessage // JSONB containment filter; e.g. {"department":"ops"}
	Limit    int
	Offset   int
}

// FilterResult is a paginated property filter response.
type FilterResult struct {
	Items  []*MergedEntityView `json:"items"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// ChangelogEntry mirrors an audit_log row, returned by the changelog endpoint (FR-RET-003).
type ChangelogEntry struct {
	ID           uuid.UUID       `json:"id"`
	Actor        string          `json:"actor"`
	Operation    string          `json:"operation"`
	ResourceType string          `json:"resource_type"`
	ResourceID   uuid.UUID       `json:"resource_id"`
	BeforeValue  json.RawMessage `json:"before_value,omitempty"`
	AfterValue   json.RawMessage `json:"after_value,omitempty"`
	SystemID     *uuid.UUID      `json:"system_id,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
}

// ChangelogParams controls changelog queries.
type ChangelogParams struct {
	Since    time.Time  // mandatory lower bound on audit_log.timestamp
	SystemID *uuid.UUID // optional; when set, returns only events for that system plus global events
	Limit    int
	Offset   int
}

// ChangelogResult is a paginated changelog response.
type ChangelogResult struct {
	Items  []*ChangelogEntry `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// ExportItem bundles an entity with its visible relations for bulk export (FR-RET-004).
type ExportItem struct {
	ID         uuid.UUID       `json:"id"`
	TypeID     uuid.UUID       `json:"type_id"`
	Type       string          `json:"type"`
	ExternalID string          `json:"external_id"`
	Properties json.RawMessage `json:"properties"`
	Relations  []RelationRef   `json:"relations"`
	SystemID   *uuid.UUID      `json:"system_id,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// ExportParams controls export queries.
type ExportParams struct {
	TypeName string     // optional type filter
	SystemID *uuid.UUID // auth context for relation scoping (set from caller's identity)
	Limit    int
	Offset   int
}

// ExportResult is a paginated export response.
type ExportResult struct {
	Items  []*ExportItem `json:"items"`
	Total  int64         `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// LogEntry is persisted to retrieval_log after each successful retrieval operation (FR-AUD-002).
type LogEntry struct {
	CallerIdentity string
	QueryParams    json.RawMessage
	ReturnedRefs   json.RawMessage // JSON array of UUID strings
	SystemID       *string
}

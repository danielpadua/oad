// Package overlay manages property overlays — system-specific properties
// layered on top of global entities.
// Each overlay is scoped to a (entity, system) pair and validated against
// the system overlay schema declared for that combination.
// Implements FR-OVL-001 through FR-OVL-004 and FR-OVL-008.
package overlay

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PropertyOverlay holds system-specific properties attached to a global entity.
// Property keys must be namespaced with the owning system name (e.g., "credit.max_approval").
type PropertyOverlay struct {
	ID         uuid.UUID       `json:"id"`
	EntityID   uuid.UUID       `json:"entity_id"`
	SystemID   uuid.UUID       `json:"system_id"`
	Properties json.RawMessage `json:"properties"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// CreateRequest is the payload for attaching overlay properties to an entity.
// The system_id is derived from the authenticated caller's token (FR-OVL-008).
type CreateRequest struct {
	Properties json.RawMessage `json:"properties"`
}

// UpdateRequest replaces the overlay properties for an existing overlay.
type UpdateRequest struct {
	Properties json.RawMessage `json:"properties"`
}

// ListParams controls pagination for overlay list queries.
type ListParams struct {
	Limit  int
	Offset int
}

// ListResult holds a page of overlays with pagination metadata.
type ListResult struct {
	Items  []*PropertyOverlay `json:"items"`
	Total  int64              `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

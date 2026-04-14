// Package overlayschema manages system overlay schemas — the per-system,
// per-entity-type schema registry that governs which overlay properties
// a system may attach to entities and enforces namespace conventions.
// Implements FR-OVS-001 through FR-OVS-005.
package overlayschema

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SystemOverlaySchema declares which overlay properties a specific system
// is permitted to attach to entities of a given type.
// All declared property keys must be prefixed with the system name
// (e.g., "credit.max_approval") to prevent key collisions (FR-OVS-005).
type SystemOverlaySchema struct {
	ID                       uuid.UUID       `json:"id"`
	SystemID                 uuid.UUID       `json:"system_id"`
	EntityTypeID             uuid.UUID       `json:"entity_type_id"`
	AllowedOverlayProperties json.RawMessage `json:"allowed_overlay_properties"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

// CreateRequest is the payload for creating a new system overlay schema.
type CreateRequest struct {
	EntityTypeID             uuid.UUID       `json:"entity_type_id"`
	AllowedOverlayProperties json.RawMessage `json:"allowed_overlay_properties"`
}

// UpdateRequest replaces the allowed_overlay_properties of an existing schema.
type UpdateRequest struct {
	AllowedOverlayProperties json.RawMessage `json:"allowed_overlay_properties"`
}

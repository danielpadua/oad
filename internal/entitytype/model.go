// Package entitytype manages entity type definitions — the dynamic schema
// registry that controls what entity types exist and constrains their
// structure at the ingestion boundary without requiring DB migrations.
// Implements FR-ETD-001 through FR-ETD-004 and NFR-EXT-001.
package entitytype

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Scope enumerates the valid values for the entity type scope column.
const (
	ScopeGlobal       = "global"
	ScopeSystemScoped = "system_scoped"
)

// EntityTypeDefinition is a schema registry entry that governs what entities
// can exist and validates their properties at every write.
type EntityTypeDefinition struct {
	ID                uuid.UUID       `json:"id"`
	TypeName          string          `json:"type_name"`
	AllowedProperties json.RawMessage `json:"allowed_properties"`
	AllowedRelations  json.RawMessage `json:"allowed_relations"`
	Scope             string          `json:"scope"` // "global" | "system_scoped"
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// CreateRequest is the payload for creating a new entity type definition.
type CreateRequest struct {
	TypeName          string          `json:"type_name"`
	AllowedProperties json.RawMessage `json:"allowed_properties"`
	AllowedRelations  json.RawMessage `json:"allowed_relations"`
	Scope             string          `json:"scope"`
}

// UpdateRequest replaces the mutable fields of an existing entity type
// definition. TypeName and Scope are immutable after creation.
type UpdateRequest struct {
	AllowedProperties json.RawMessage `json:"allowed_properties"`
	AllowedRelations  json.RawMessage `json:"allowed_relations"`
}

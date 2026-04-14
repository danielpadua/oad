// Package entity manages typed nodes in the authorization graph.
// Entities represent subjects, resources, roles, groups, or any typed object.
// Implements FR-ENT-001 through FR-ENT-008.
package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Entity is a typed node in the authorization graph.
type Entity struct {
	ID         uuid.UUID       `json:"id"`
	TypeID     uuid.UUID       `json:"type_id"`
	Type       string          `json:"type"` // type_name resolved from entity_type_definition
	ExternalID string          `json:"external_id"`
	Properties json.RawMessage `json:"properties"`
	SystemID   *uuid.UUID      `json:"system_id,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// CreateRequest is the payload for creating a new entity.
type CreateRequest struct {
	Type       string          `json:"type"`
	ExternalID string          `json:"external_id"`
	Properties json.RawMessage `json:"properties,omitempty"`
	SystemID   *uuid.UUID      `json:"system_id,omitempty"`
}

// UpdateRequest is the payload for updating an entity's properties (FR-ENT-005).
// Only properties may be updated; type, external_id, and system_id are immutable.
type UpdateRequest struct {
	Properties json.RawMessage `json:"properties"`
}

// ListParams controls filtering and pagination for entity list queries.
type ListParams struct {
	TypeName string // filter by type_name (optional)
	Limit    int
	Offset   int
}

// ListResult holds a page of entities with pagination metadata.
type ListResult struct {
	Items  []*Entity `json:"items"`
	Total  int64     `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

// BulkCreateRequest is the payload for bulk entity import (FR-ENT-007).
type BulkCreateRequest struct {
	Entities []CreateRequest `json:"entities"`
	// Mode controls whether existing entities are updated ("upsert") or cause
	// an error ("create"). Defaults to "create".
	Mode string `json:"mode"`
}

// BulkCreateResult summarises the outcome of a bulk import operation.
type BulkCreateResult struct {
	Total   int             `json:"total"`
	Created int             `json:"created"`
	Updated int             `json:"updated"` // populated in upsert mode only
	Errors  []BulkItemError `json:"errors"`
}

// BulkItemError describes a failure for one item within a bulk operation.
type BulkItemError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

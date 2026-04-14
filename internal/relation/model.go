// Package relation manages directed edges between entities in the authorization
// graph. Relations are the building block for RBAC and ReBAC policy evaluation.
// Implements FR-REL-001 through FR-REL-005.
package relation

import (
	"time"

	"github.com/google/uuid"
)

// Relation is a typed, directed edge between two entities.
type Relation struct {
	ID              uuid.UUID  `json:"id"`
	SubjectEntityID uuid.UUID  `json:"subject_entity_id"`
	RelationType    string     `json:"relation_type"`
	TargetEntityID  uuid.UUID  `json:"target_entity_id"`
	SystemID        *uuid.UUID `json:"system_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateRequest is the payload for creating a new relation (FR-REL-001).
type CreateRequest struct {
	SubjectEntityID uuid.UUID `json:"subject_entity_id"`
	RelationType    string    `json:"relation_type"`
	TargetEntityID  uuid.UUID `json:"target_entity_id"`
	// SystemID, when set, creates a system-scoped relation visible only within
	// that system's context. Nil creates a global relation (FR-OVL-005).
	SystemID *uuid.UUID `json:"system_id,omitempty"`
}

// ListParams controls filtering and pagination for relation list queries.
type ListParams struct {
	// RelationType, when non-empty, restricts results to this relation type.
	RelationType string
	// SystemID, when set, restricts results to system-scoped relations for that system.
	// Nil returns all relations visible to the caller (global + in-scope via RLS).
	SystemID *uuid.UUID
	Limit    int
	Offset   int
}

// ListResult holds a page of relations with pagination metadata.
type ListResult struct {
	Items  []*Relation `json:"items"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

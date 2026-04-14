// Package system manages registered applications (systems) that participate
// in OAD authorization data management.
// Implements FR-SYS-001 through FR-SYS-003.
package system

import (
	"time"

	"github.com/google/uuid"
)

// System represents a registered application whose authorization data
// is managed in OAD. It defines the management boundary for product teams.
type System struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateRequest is the payload for registering a new system.
type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PatchRequest carries optional field updates for an existing system.
// Pointer fields distinguish "not provided" from "set to zero value".
// Setting Active to false deactivates the system (FR-SYS-003).
type PatchRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Active      *bool   `json:"active,omitempty"`
}

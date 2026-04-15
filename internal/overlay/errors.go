package overlay

import "errors"

// Sentinel errors returned by the repository layer.
// The service maps these to structured *apierr.APIError values.
var (
	// ErrNotFound is returned when an overlay record does not exist.
	ErrNotFound = errors.New("property overlay not found")

	// ErrDuplicate is returned when an overlay already exists for the
	// same (entity_id, system_id) pair (UNIQUE constraint violation).
	ErrDuplicate = errors.New("property overlay already exists for this entity and system")

	// ErrEntityNotFound is returned when the referenced entity does not exist.
	ErrEntityNotFound = errors.New("entity not found")

	// ErrNoSchema is returned when no system overlay schema is declared for
	// the given (system_id, entity_type_id) combination (FR-OVL-003).
	ErrNoSchema = errors.New("no system overlay schema declared for this system and entity type")

	// ErrNoSystemScope is returned when a write operation is attempted by a
	// caller without a system scope (platform admins cannot own overlays).
	ErrNoSystemScope = errors.New("a system scope is required to manage property overlays")
)

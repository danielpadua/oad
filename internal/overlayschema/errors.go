package overlayschema

import "errors"

var (
	// ErrNotFound is returned when the requested overlay schema does not exist.
	ErrNotFound = errors.New("system overlay schema not found")

	// ErrDuplicate is returned when (system_id, entity_type_id) already has a schema.
	ErrDuplicate = errors.New("system overlay schema already exists for this system and entity type")

	// ErrSystemNotFound is returned when the referenced system does not exist.
	ErrSystemNotFound = errors.New("system not found")

	// ErrEntityTypeNotFound is returned when the referenced entity type does not exist.
	ErrEntityTypeNotFound = errors.New("entity type definition not found")

	// ErrHasOverlays is returned when deletion is attempted while property overlays
	// validated by this schema still exist (FR-OVS-003).
	ErrHasOverlays = errors.New("system overlay schema has associated property overlays and cannot be deleted")
)

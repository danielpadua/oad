package entitytype

import "errors"

var (
	// ErrNotFound is returned when the requested entity type definition does not exist.
	ErrNotFound = errors.New("entity type definition not found")

	// ErrDuplicateTypeName is returned when the type_name is already registered.
	ErrDuplicateTypeName = errors.New("entity type name already exists")

	// ErrHasEntities is returned when deletion is attempted while entities
	// of this type still exist (FR-ETD-003).
	ErrHasEntities = errors.New("entity type definition has associated entities and cannot be deleted")
)

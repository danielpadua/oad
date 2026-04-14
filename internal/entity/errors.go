package entity

import "errors"

var (
	// ErrNotFound is returned when the requested entity does not exist.
	ErrNotFound = errors.New("entity not found")

	// ErrDuplicateExternalID is returned when (type_id, external_id) already exists.
	ErrDuplicateExternalID = errors.New("entity with this type and external_id already exists")
)

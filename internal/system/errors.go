package system

import "errors"

var (
	// ErrNotFound is returned when the requested system does not exist.
	ErrNotFound = errors.New("system not found")

	// ErrDuplicateName is returned when the system name is already registered.
	ErrDuplicateName = errors.New("system name already exists")
)

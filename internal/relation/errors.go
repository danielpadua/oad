package relation

import "errors"

var (
	// ErrNotFound is returned when the requested relation does not exist.
	ErrNotFound = errors.New("relation not found")

	// ErrDuplicate is returned when the same (subject, relation_type, target, system_id)
	// tuple already exists, preventing duplicate edges in the authorization graph.
	ErrDuplicate = errors.New("duplicate relation")
)

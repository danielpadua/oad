package retrieval

import "errors"

// ErrEntityNotFound is returned when the requested entity does not exist.
var ErrEntityNotFound = errors.New("entity not found")

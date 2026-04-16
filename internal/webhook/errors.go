package webhook

import "errors"

var (
	// ErrNotFound is returned when a webhook subscription record does not exist.
	ErrNotFound = errors.New("webhook subscription not found")

	// ErrNoSystemScope is returned when a write operation is attempted by a
	// caller without a system scope. Platform admins must specify a system via
	// the path parameter; system-scoped callers derive their scope from the token.
	ErrNoSystemScope = errors.New("a system scope is required to manage webhook subscriptions")
)

package groups

import "errors"

var (
	// ErrNotFound indicates that no group is linked to the given (provider,
	// entity_id) pair. Translated to 404 by the handler layer.
	ErrNotFound = errors.New("scim group not found")

	// ErrAlreadyExists indicates a (provider, external_subject) collision
	// on create. Translated to 409 (uniqueness) by the handler layer.
	ErrAlreadyExists = errors.New("scim group already exists for this provider+externalId")

	// ErrDisplayNameRequired indicates the displayName attribute is missing.
	// Translated to 400 by the handler layer.
	ErrDisplayNameRequired = errors.New("scim displayName is required")

	// ErrExternalIDRequired indicates the externalId attribute is missing
	// on create. OAD requires it so it can link the SCIM resource to the
	// IdP's stable identifier. Translated to 400.
	ErrExternalIDRequired = errors.New("scim externalId is required on create")

	// ErrInvalidMember indicates a member.value is not a UUID, references
	// an entity that does not exist, references an entity owned by a
	// different provider, or references an entity whose type is not User
	// or Group. Translated to 400.
	ErrInvalidMember = errors.New("scim group member is invalid")

	// ErrBuiltinGroup indicates an attempt to mutate or delete a reserved
	// built-in group (oad:admin, oad:editor, oad:viewer). These are seeded
	// rows protected from API write operations. Translated to 403.
	ErrBuiltinGroup = errors.New("scim group is built-in and cannot be modified via SCIM")

	// ErrInvalidFilter wraps parser/translator errors so the handler can
	// translate them to RFC 7644 §3.4.2.2 invalidFilter responses.
	ErrInvalidFilter = errors.New("scim filter is invalid")
)

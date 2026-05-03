package users

import "errors"

var (
	// ErrNotFound indicates that no user is linked to the given (provider,
	// entity_id) pair. Translated to 404 by the handler layer.
	ErrNotFound = errors.New("scim user not found")

	// ErrAlreadyExists indicates a (provider, external_subject) collision on
	// create. Translated to 409 (uniqueness) by the handler layer.
	ErrAlreadyExists = errors.New("scim user already exists for this provider+externalId")

	// ErrUserNameRequired indicates that the userName attribute is missing
	// from the SCIM payload. Translated to 400 by the handler layer.
	ErrUserNameRequired = errors.New("scim userName is required")

	// ErrExternalIDRequired indicates the externalId attribute is missing on
	// create. OAD requires it so it can link the SCIM resource to the IdP's
	// stable identifier. Translated to 400.
	ErrExternalIDRequired = errors.New("scim externalId is required on create")

	// ErrInvalidFilter wraps parser/translator errors so the handler can
	// translate them to RFC 7644 §3.4.2.2 invalidFilter responses.
	ErrInvalidFilter = errors.New("scim filter is invalid")
)

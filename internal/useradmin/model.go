package useradmin

import (
	"github.com/danielpadua/oad/internal/entity"
)

// ExternalIdentity represents a linked external IdP identity.
type ExternalIdentity struct {
	ProviderName    string `json:"provider_name"`
	ExternalSubject string `json:"external_subject"`
}

// User extends the base Entity with a list of external identities.
type User struct {
	*entity.Entity
	ExternalIdentities []ExternalIdentity `json:"external_identities"`
}

// ListParams controls pagination for user listing.
type ListParams struct {
	Limit  int
	Offset int
}

// ListResult holds a paginated list of users.
type ListResult struct {
	Items  []*User `json:"items"`
	Total  int64   `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

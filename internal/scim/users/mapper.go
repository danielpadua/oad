package users

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PropertyKeyUserName is the entity.properties JSONB key for the SCIM userName.
const (
	PropertyKeyUserName    = "userName"
	PropertyKeyDisplayName = "displayName"
	PropertyKeyEmail       = "email"
	PropertyKeyActive      = "active"

	resourceTypeName = "User"
)

// StoredUser is the persistence-layer view of a SCIM User. Properties is
// the entity.properties JSONB decoded into a map. ExternalSubject is the
// entity_external_identity.external_subject for the caller's provider.
type StoredUser struct {
	EntityID        uuid.UUID
	Properties      map[string]any
	ExternalSubject string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ToProperties converts an inbound SCIM User into the entity.properties
// JSONB shape that the User entity_type_definition validates against.
// Fields not declared in the schema are dropped (RFC 7643 — server may
// silently ignore unknown attributes at the ingestion boundary).
func ToProperties(u User) map[string]any {
	props := map[string]any{
		PropertyKeyUserName: u.UserName,
		PropertyKeyActive:   derefBoolDefault(u.Active, true),
	}
	if dn := pickDisplayName(u); dn != "" {
		props[PropertyKeyDisplayName] = dn
	}
	if email := pickPrimaryEmail(u.Emails); email != "" {
		props[PropertyKeyEmail] = email
	}
	return props
}

// FromStored renders a StoredUser as an outbound SCIM User. ETag (Meta.Version)
// is computed deterministically from entity_id + updated_at.
func FromStored(s StoredUser) User {
	u := User{
		Schemas:    []string{SchemaURN},
		ID:         s.EntityID.String(),
		ExternalID: s.ExternalSubject,
		UserName:   asString(s.Properties[PropertyKeyUserName]),
	}
	if dn := asString(s.Properties[PropertyKeyDisplayName]); dn != "" {
		u.DisplayName = dn
	}
	if email := asString(s.Properties[PropertyKeyEmail]); email != "" {
		u.Emails = []Email{{Value: email, Primary: true}}
	}
	if v, ok := s.Properties[PropertyKeyActive].(bool); ok {
		active := v
		u.Active = &active
	}
	u.Meta = &Meta{
		ResourceType: resourceTypeName,
		Created:      s.CreatedAt,
		LastModified: s.UpdatedAt,
		Location:     "/scim/v2/Users/" + s.EntityID.String(),
		Version:      ETag(s.EntityID, s.UpdatedAt),
	}
	return u
}

// ETag returns the SCIM weak ETag for a User row. Computed as
// W/"<sha256(id || updated_at_rfc3339nano)>" truncated to 16 hex chars.
func ETag(id uuid.UUID, updatedAt time.Time) string {
	h := sha256.Sum256([]byte(id.String() + updatedAt.UTC().Format(time.RFC3339Nano)))
	return fmt.Sprintf("W/%q", hex.EncodeToString(h[:8]))
}

func pickDisplayName(u User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Name != nil && u.Name.Formatted != "" {
		return u.Name.Formatted
	}
	return ""
}

func pickPrimaryEmail(emails []Email) string {
	if len(emails) == 0 {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Value != "" {
			return e.Value
		}
	}
	if emails[0].Value != "" {
		return emails[0].Value
	}
	return ""
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func derefBoolDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

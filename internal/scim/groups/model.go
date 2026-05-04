// Package groups implements the SCIM 2.0 Group resource. Each Group maps
// to an entity of type 'Group' plus a row in entity_external_identity for
// the calling provider; membership is persisted as relation rows of type
// 'member_of' from the member entity to the group entity.
package groups

import (
	"time"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/scim/schema"
)

// Group is the JSON-serializable SCIM 2.0 Group resource. Field tags follow
// RFC 7643 §4.2; only the subset OAD persists is exposed. Clients may
// send other standard SCIM attributes; they are dropped at the ingestion
// boundary.
type Group struct {
	Schemas     []string `json:"schemas,omitempty"`
	ID          string   `json:"id,omitempty"`
	ExternalID  string   `json:"externalId,omitempty"`
	DisplayName string   `json:"displayName"`
	Members     []Member `json:"members,omitempty"`
	Meta        *Meta    `json:"meta,omitempty"`
}

// Member is one entry in the SCIM Group.members multi-valued attribute.
// Value is the SCIM id (entity UUID) of an existing User or Group
// resource. Ref, Type, and Display are derived at read time and ignored
// on write.
type Member struct {
	Value   string `json:"value"`
	Ref     string `json:"$ref,omitempty"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

// Meta carries SCIM resource metadata. ETag is exposed as Version per
// RFC 7644 §3.14.
type Meta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Location     string    `json:"location"`
	Version      string    `json:"version,omitempty"`
}

// SchemaURN is re-exported from the schema package for convenience.
const SchemaURN = schema.GroupSchemaURN

// MemberEntity is a lightweight projection of a member entity returned
// by the repository. Type is the entity_type_definition.type_name (User
// or Group) and Display is the displayName property when present.
type MemberEntity struct {
	EntityID uuid.UUID
	Type     string
	Display  string
}

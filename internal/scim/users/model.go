// Package users implements the SCIM 2.0 User resource. The package exposes
// a Service that backends User operations against the entity / relation
// graph (each User maps to an entity of type 'User' plus a row in
// entity_external_identity for the calling provider).
package users

import (
	"time"

	"github.com/danielpadua/oad/internal/scim/schema"
)

// User is the JSON-serializable SCIM 2.0 User resource. Field tags follow
// RFC 7643 §4.1; only the subset OAD persists is exposed. Clients may
// send other standard SCIM attributes; they are dropped at the ingestion
// boundary.
type User struct {
	Schemas     []string `json:"schemas,omitempty"`
	ID          string   `json:"id,omitempty"`
	ExternalID  string   `json:"externalId,omitempty"`
	UserName    string   `json:"userName"`
	DisplayName string   `json:"displayName,omitempty"`
	Name        *Name    `json:"name,omitempty"`
	Emails      []Email  `json:"emails,omitempty"`
	Active      *bool    `json:"active,omitempty"`
	Meta        *Meta    `json:"meta,omitempty"`
}

// Name is the SCIM "complex name" sub-resource. OAD reads name.formatted
// as a fallback for displayName when displayName is empty.
type Name struct {
	Formatted  string `json:"formatted,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
}

// Email is one entry in the SCIM User.emails multi-valued attribute.
type Email struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
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

// SchemaURN is re-exported from the schema package for convenience at
// SCIM-related call sites.
const SchemaURN = schema.UserSchemaURN

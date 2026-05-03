// Package schema declares the SCIM 2.0 schemas and resource types that OAD
// supports. The values are returned by the discovery endpoints and are
// authoritative for what attributes resource handlers accept and emit.
package schema

const (
	// UserSchemaURN is the SCIM 2.0 core schema URN for User resources.
	UserSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:User"

	// SchemaURN is the SCIM 2.0 schema URN for schema definitions.
	SchemaURN = "urn:ietf:params:scim:schemas:core:2.0:Schema"

	// ResourceTypeURN is the SCIM 2.0 schema URN for ResourceType definitions.
	ResourceTypeURN = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
)

// Attribute describes one attribute in a SCIM schema (RFC 7643 §7).
type Attribute struct {
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	MultiValued   bool        `json:"multiValued"`
	Required      bool        `json:"required"`
	CaseExact     bool        `json:"caseExact,omitempty"`
	Mutability    string      `json:"mutability"`
	Returned      string      `json:"returned"`
	Uniqueness    string      `json:"uniqueness,omitempty"`
	Description   string      `json:"description,omitempty"`
	SubAttributes []Attribute `json:"subAttributes,omitempty"`
}

// Schema is the JSON shape returned by GET /scim/v2/Schemas/{id}.
type Schema struct {
	Schemas     []string    `json:"schemas"`
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Attributes  []Attribute `json:"attributes"`
	Meta        SchemaMeta  `json:"meta"`
}

// SchemaMeta is the meta block for a Schema response.
type SchemaMeta struct {
	ResourceType string `json:"resourceType"`
	Location     string `json:"location"`
}

// ResourceType is the JSON shape returned by GET /scim/v2/ResourceTypes/{id}.
type ResourceType struct {
	Schemas     []string         `json:"schemas"`
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Endpoint    string           `json:"endpoint"`
	Description string           `json:"description,omitempty"`
	Schema      string           `json:"schema"`
	Meta        ResourceTypeMeta `json:"meta"`
}

// ResourceTypeMeta is the meta block for a ResourceType response.
type ResourceTypeMeta struct {
	ResourceType string `json:"resourceType"`
	Location     string `json:"location"`
}

// User returns the SCIM 2.0 User schema OAD advertises. Only the attributes
// OAD persists are declared; clients may send other standard fields, which
// are silently dropped at the ingestion boundary.
func User() Schema {
	return Schema{
		Schemas:     []string{SchemaURN},
		ID:          UserSchemaURN,
		Name:        "User",
		Description: "User Account",
		Attributes: []Attribute{
			{
				Name:        "userName",
				Type:        "string",
				MultiValued: false,
				Required:    true,
				CaseExact:   false,
				Mutability:  "readWrite",
				Returned:    "default",
				Uniqueness:  "server",
				Description: "Unique identifier for the user, typically an email address or login name.",
			},
			{
				Name:        "displayName",
				Type:        "string",
				MultiValued: false,
				Required:    false,
				CaseExact:   false,
				Mutability:  "readWrite",
				Returned:    "default",
				Uniqueness:  "none",
				Description: "Human-readable display name.",
			},
			{
				Name:        "active",
				Type:        "boolean",
				MultiValued: false,
				Required:    false,
				Mutability:  "readWrite",
				Returned:    "default",
				Description: "Whether the user is active. Inactive users are denied authentication.",
			},
			{
				Name:        "emails",
				Type:        "complex",
				MultiValued: true,
				Required:    false,
				Mutability:  "readWrite",
				Returned:    "default",
				Description: "Email addresses for the user. The first entry with primary=true is persisted.",
				SubAttributes: []Attribute{
					{
						Name:        "value",
						Type:        "string",
						MultiValued: false,
						Required:    false,
						CaseExact:   false,
						Mutability:  "readWrite",
						Returned:    "default",
						Uniqueness:  "none",
					},
					{
						Name:        "primary",
						Type:        "boolean",
						MultiValued: false,
						Required:    false,
						Mutability:  "readWrite",
						Returned:    "default",
					},
				},
			},
		},
		Meta: SchemaMeta{
			ResourceType: "Schema",
			Location:     "/scim/v2/Schemas/" + UserSchemaURN,
		},
	}
}

// UserResourceType returns the SCIM 2.0 ResourceType describing the User
// resource as exposed by OAD.
func UserResourceType() ResourceType {
	return ResourceType{
		Schemas:     []string{ResourceTypeURN},
		ID:          "User",
		Name:        "User",
		Endpoint:    "/Users",
		Description: "User Account",
		Schema:      UserSchemaURN,
		Meta: ResourceTypeMeta{
			ResourceType: "ResourceType",
			Location:     "/scim/v2/ResourceTypes/User",
		},
	}
}

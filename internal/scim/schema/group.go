package schema

// GroupSchemaURN is the SCIM 2.0 core schema URN for Group resources.
const GroupSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:Group"

// Group returns the SCIM 2.0 Group schema OAD advertises. Membership is
// stored as relations (member --member_of--> group), so the SCIM members
// attribute is computed at read time and translated to relations at write
// time. Only attributes OAD persists are declared.
func Group() Schema {
	return Schema{
		Schemas:     []string{SchemaURN},
		ID:          GroupSchemaURN,
		Name:        "Group",
		Description: "Group",
		Attributes: []Attribute{
			{
				Name:        "displayName",
				Type:        "string",
				MultiValued: false,
				Required:    true,
				CaseExact:   false,
				Mutability:  "readWrite",
				Returned:    "default",
				Uniqueness:  "none",
				Description: "Human-readable name for the Group.",
			},
			{
				Name:        "members",
				Type:        "complex",
				MultiValued: true,
				Required:    false,
				Mutability:  "readWrite",
				Returned:    "default",
				Description: "Members of the Group. value is the SCIM id of an existing User or Group resource provisioned by the same provider.",
				SubAttributes: []Attribute{
					{
						Name:        "value",
						Type:        "string",
						MultiValued: false,
						Required:    true,
						CaseExact:   true,
						Mutability:  "immutable",
						Returned:    "default",
						Uniqueness:  "none",
					},
					{
						Name:        "$ref",
						Type:        "reference",
						MultiValued: false,
						Required:    false,
						CaseExact:   true,
						Mutability:  "readOnly",
						Returned:    "default",
					},
					{
						Name:        "type",
						Type:        "string",
						MultiValued: false,
						Required:    false,
						CaseExact:   false,
						Mutability:  "readOnly",
						Returned:    "default",
						Description: "Type of the member. Either User or Group.",
					},
					{
						Name:        "display",
						Type:        "string",
						MultiValued: false,
						Required:    false,
						CaseExact:   false,
						Mutability:  "readOnly",
						Returned:    "default",
					},
				},
			},
		},
		Meta: SchemaMeta{
			ResourceType: "Schema",
			Location:     "/scim/v2/Schemas/" + GroupSchemaURN,
		},
	}
}

// GroupResourceType returns the SCIM 2.0 ResourceType describing the Group
// resource as exposed by OAD.
func GroupResourceType() ResourceType {
	return ResourceType{
		Schemas:     []string{ResourceTypeURN},
		ID:          "Group",
		Name:        "Group",
		Endpoint:    "/Groups",
		Description: "Group",
		Schema:      GroupSchemaURN,
		Meta: ResourceTypeMeta{
			ResourceType: "ResourceType",
			Location:     "/scim/v2/ResourceTypes/Group",
		},
	}
}

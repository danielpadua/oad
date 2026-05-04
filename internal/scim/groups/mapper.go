package groups

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Property keys for the entity.properties JSONB document. The Group
// entity_type_definition declares only displayName and description; OAD
// persists displayName from SCIM and leaves description for admin-managed
// groups.
const (
	PropertyKeyDisplayName = "displayName"
	PropertyKeyDescription = "description"

	resourceTypeName = "Group"
)

// StoredGroup is the persistence-layer view of a SCIM Group. Properties is
// the entity.properties JSONB decoded into a map. Members carries the
// projection of member entities resolved by the repository for outbound
// rendering. ExternalSubject is the entity_external_identity.external_subject
// for the caller's provider.
type StoredGroup struct {
	EntityID        uuid.UUID
	Properties      map[string]any
	ExternalSubject string
	Members         []MemberEntity
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ToProperties converts an inbound SCIM Group into the entity.properties
// JSONB shape that the Group entity_type_definition validates against.
// Members are NOT persisted as properties — they live in the relation
// table (see service.applyMembers).
func ToProperties(g Group) map[string]any {
	props := map[string]any{
		PropertyKeyDisplayName: g.DisplayName,
	}
	return props
}

// FromStored renders a StoredGroup as an outbound SCIM Group. ETag
// (Meta.Version) is computed deterministically from entity_id +
// updated_at. Members are rendered with $ref pointing at the relevant
// resource endpoint.
func FromStored(s StoredGroup) Group {
	g := Group{
		Schemas:     []string{SchemaURN},
		ID:          s.EntityID.String(),
		ExternalID:  s.ExternalSubject,
		DisplayName: asString(s.Properties[PropertyKeyDisplayName]),
	}

	if len(s.Members) > 0 {
		g.Members = make([]Member, 0, len(s.Members))
		for _, m := range s.Members {
			g.Members = append(g.Members, Member{
				Value:   m.EntityID.String(),
				Ref:     memberRef(m),
				Type:    m.Type,
				Display: m.Display,
			})
		}
	}

	g.Meta = &Meta{
		ResourceType: resourceTypeName,
		Created:      s.CreatedAt,
		LastModified: s.UpdatedAt,
		Location:     "/scim/v2/Groups/" + s.EntityID.String(),
		Version:      ETag(s.EntityID, s.UpdatedAt),
	}
	return g
}

// ETag returns the SCIM weak ETag for a Group row. Computed as
// W/"<sha256(id || updated_at_rfc3339nano)>" truncated to 16 hex chars.
func ETag(id uuid.UUID, updatedAt time.Time) string {
	h := sha256.Sum256([]byte(id.String() + updatedAt.UTC().Format(time.RFC3339Nano)))
	return fmt.Sprintf("W/%q", hex.EncodeToString(h[:8]))
}

// memberRef builds the SCIM $ref URL fragment for a member based on its
// type. Group $refs target /Groups/<id>; everything else targets
// /Users/<id>.
func memberRef(m MemberEntity) string {
	endpoint := "/scim/v2/Users/"
	if m.Type == "Group" {
		endpoint = "/scim/v2/Groups/"
	}
	return endpoint + m.EntityID.String()
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

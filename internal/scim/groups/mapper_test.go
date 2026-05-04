package groups_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/scim/groups"
)

func TestToProperties_DropsMembers(t *testing.T) {
	t.Parallel()

	in := groups.Group{
		DisplayName: "Engineering",
		Members: []groups.Member{
			{Value: uuid.NewString()},
		},
	}
	props := groups.ToProperties(in)
	if got := props[groups.PropertyKeyDisplayName]; got != "Engineering" {
		t.Errorf("displayName = %v, want Engineering", got)
	}
	if _, ok := props["members"]; ok {
		t.Errorf("ToProperties leaked members into properties: %v", props)
	}
}

func TestFromStored_BasicShape(t *testing.T) {
	t.Parallel()

	gid := uuid.New()
	mid := uuid.New()
	created := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Minute)

	out := groups.FromStored(groups.StoredGroup{
		EntityID:        gid,
		Properties:      map[string]any{groups.PropertyKeyDisplayName: "Engineering"},
		ExternalSubject: "eng-1",
		Members: []groups.MemberEntity{
			{EntityID: mid, Type: "User", Display: "Alice"},
		},
		CreatedAt: created,
		UpdatedAt: updated,
	})

	if out.ID != gid.String() {
		t.Errorf("id = %q, want %q", out.ID, gid.String())
	}
	if out.ExternalID != "eng-1" {
		t.Errorf("externalId = %q, want eng-1", out.ExternalID)
	}
	if out.DisplayName != "Engineering" {
		t.Errorf("displayName = %q, want Engineering", out.DisplayName)
	}
	if len(out.Members) != 1 {
		t.Fatalf("members count = %d, want 1", len(out.Members))
	}
	m := out.Members[0]
	if m.Value != mid.String() {
		t.Errorf("member.value = %q, want %q", m.Value, mid.String())
	}
	if m.Type != "User" {
		t.Errorf("member.type = %q, want User", m.Type)
	}
	if !strings.HasPrefix(m.Ref, "/scim/v2/Users/") {
		t.Errorf("member.$ref = %q, want /scim/v2/Users/ prefix", m.Ref)
	}
	if m.Display != "Alice" {
		t.Errorf("member.display = %q, want Alice", m.Display)
	}
	if out.Meta == nil || out.Meta.ResourceType != "Group" {
		t.Errorf("meta.resourceType = %v, want Group", out.Meta)
	}
	if !strings.HasPrefix(out.Meta.Location, "/scim/v2/Groups/") {
		t.Errorf("meta.location = %q, want /scim/v2/Groups/ prefix", out.Meta.Location)
	}
	if !strings.HasPrefix(out.Meta.Version, `W/"`) {
		t.Errorf("meta.version = %q, want weak ETag prefix", out.Meta.Version)
	}
}

func TestFromStored_GroupMember_RefPointsAtGroups(t *testing.T) {
	t.Parallel()

	out := groups.FromStored(groups.StoredGroup{
		EntityID:   uuid.New(),
		Properties: map[string]any{groups.PropertyKeyDisplayName: "Parent"},
		Members: []groups.MemberEntity{
			{EntityID: uuid.New(), Type: "Group", Display: "Child"},
		},
	})
	if !strings.HasPrefix(out.Members[0].Ref, "/scim/v2/Groups/") {
		t.Errorf("nested group $ref = %q, want /scim/v2/Groups/ prefix", out.Members[0].Ref)
	}
}

func TestETag_StableForSameInputs(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	if a, b := groups.ETag(id, at), groups.ETag(id, at); a != b {
		t.Errorf("ETag not deterministic: %q vs %q", a, b)
	}
}

func TestETag_ChangesWhenUpdatedAtChanges(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	a := groups.ETag(id, time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))
	b := groups.ETag(id, time.Date(2026, 5, 1, 12, 0, 1, 0, time.UTC))
	if a == b {
		t.Errorf("ETag did not change with updatedAt: %q == %q", a, b)
	}
}

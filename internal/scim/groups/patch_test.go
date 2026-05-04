package groups_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/scim/groups"
)

func mustRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func baseEngState() (groups.PatchState, []uuid.UUID) {
	m1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	m2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return groups.PatchState{
		Properties: map[string]any{
			groups.PropertyKeyDisplayName: "Engineering",
		},
		MemberIDs: []uuid.UUID{m1, m2},
	}, []uuid.UUID{m1, m2}
}

func TestApplyPatch_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	in, originalMembers := baseEngState()
	_, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op: "replace", Path: "displayName", Value: mustRaw(t, "Eng Updated"),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if in.Properties[groups.PropertyKeyDisplayName] != "Engineering" {
		t.Errorf("input properties mutated: %v", in.Properties[groups.PropertyKeyDisplayName])
	}
	if len(in.MemberIDs) != len(originalMembers) || in.MemberIDs[0] != originalMembers[0] {
		t.Errorf("input MemberIDs mutated: %v", in.MemberIDs)
	}
}

func TestApplyPatch_ReplaceDisplayName(t *testing.T) {
	t.Parallel()
	in, _ := baseEngState()
	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op: "replace", Path: "displayName", Value: mustRaw(t, "Engineering Renamed"),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out.Properties[groups.PropertyKeyDisplayName] != "Engineering Renamed" {
		t.Errorf("displayName = %v, want Engineering Renamed", out.Properties[groups.PropertyKeyDisplayName])
	}
}

func TestApplyPatch_AddDisplayName(t *testing.T) {
	t.Parallel()
	in, _ := baseEngState()
	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op: "add", Path: "displayName", Value: mustRaw(t, "Eng A"),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out.Properties[groups.PropertyKeyDisplayName] != "Eng A" {
		t.Errorf("displayName = %v, want Eng A", out.Properties[groups.PropertyKeyDisplayName])
	}
}

func TestApplyPatch_AddMembers_AppendsAndDedups(t *testing.T) {
	t.Parallel()
	in, members := baseEngState()
	new1 := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op:   "add",
		Path: "members",
		Value: mustRaw(t, []any{
			map[string]any{"value": new1.String()},
			map[string]any{"value": members[0].String()}, // already present
		}),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(out.MemberIDs) != 3 {
		t.Fatalf("MemberIDs len = %d, want 3", len(out.MemberIDs))
	}
	if out.MemberIDs[0] != members[0] || out.MemberIDs[1] != members[1] || out.MemberIDs[2] != new1 {
		t.Errorf("MemberIDs order = %v, want [%s %s %s]", out.MemberIDs, members[0], members[1], new1)
	}
}

func TestApplyPatch_AddMembers_SingleObject(t *testing.T) {
	t.Parallel()
	in, members := baseEngState()
	new1 := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op:    "add",
		Path:  "members",
		Value: mustRaw(t, map[string]any{"value": new1.String()}),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(out.MemberIDs) != 3 || out.MemberIDs[2] != new1 {
		t.Errorf("MemberIDs = %v, want appended %s after %v", out.MemberIDs, new1, members)
	}
}

func TestApplyPatch_ReplaceMembers_FullList(t *testing.T) {
	t.Parallel()
	in, _ := baseEngState()
	a := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	b := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op:   "replace",
		Path: "members",
		Value: mustRaw(t, []any{
			map[string]any{"value": a.String()},
			map[string]any{"value": b.String()},
			map[string]any{"value": a.String()}, // dup, dropped
		}),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(out.MemberIDs) != 2 || out.MemberIDs[0] != a || out.MemberIDs[1] != b {
		t.Errorf("MemberIDs = %v, want [%s %s]", out.MemberIDs, a, b)
	}
}

func TestApplyPatch_RemoveMembers_All(t *testing.T) {
	t.Parallel()
	in, _ := baseEngState()
	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op:   "remove",
		Path: "members",
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(out.MemberIDs) != 0 {
		t.Errorf("MemberIDs = %v, want empty", out.MemberIDs)
	}
}

func TestApplyPatch_RemoveMembers_ByValueFilter(t *testing.T) {
	t.Parallel()
	in, members := baseEngState()
	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op:   "remove",
		Path: `members[value eq "` + members[0].String() + `"]`,
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(out.MemberIDs) != 1 || out.MemberIDs[0] != members[1] {
		t.Errorf("MemberIDs = %v, want only %s", out.MemberIDs, members[1])
	}
}

func TestApplyPatch_RemoveMembers_AbsentIsNoop(t *testing.T) {
	t.Parallel()
	in, members := baseEngState()
	missing := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	out, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op:   "remove",
		Path: `members[value eq "` + missing.String() + `"]`,
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(out.MemberIDs) != 2 || out.MemberIDs[0] != members[0] || out.MemberIDs[1] != members[1] {
		t.Errorf("MemberIDs = %v, want unchanged %v", out.MemberIDs, members)
	}
}

func TestApplyPatch_OperationsAppliedInOrder(t *testing.T) {
	t.Parallel()
	in, _ := baseEngState()
	new1 := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	out, err := groups.ApplyPatch(in, []groups.PatchOp{
		{Op: "replace", Path: "displayName", Value: mustRaw(t, "First")},
		{Op: "replace", Path: "displayName", Value: mustRaw(t, "Second")},
		{Op: "add", Path: "members", Value: mustRaw(t, []any{map[string]any{"value": new1.String()}})},
	})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out.Properties[groups.PropertyKeyDisplayName] != "Second" {
		t.Errorf("displayName = %v, want Second", out.Properties[groups.PropertyKeyDisplayName])
	}
	if len(out.MemberIDs) != 3 || out.MemberIDs[2] != new1 {
		t.Errorf("MemberIDs = %v, want trailing %s", out.MemberIDs, new1)
	}
}

func TestApplyPatch_NoTargetCases(t *testing.T) {
	t.Parallel()
	cases := []groups.PatchOp{
		// Unsupported op.
		{Op: "move", Path: "displayName", Value: mustRaw(t, "x")},
		// Remove displayName not allowed.
		{Op: "remove", Path: "displayName"},
		// Sub-attr on displayName not allowed.
		{Op: "replace", Path: "displayName.text", Value: mustRaw(t, "x")},
		// Filter on displayName not allowed.
		{Op: "replace", Path: `displayName[primary eq true]`, Value: mustRaw(t, "x")},
		// Wrong filter shape on members.
		{Op: "remove", Path: `members[type eq "User"]`},
		// Sub-attr on members remove not supported.
		{Op: "remove", Path: `members[value eq "11111111-1111-1111-1111-111111111111"].display`},
		// Add on unknown attribute.
		{Op: "add", Path: "externalId", Value: mustRaw(t, "x")},
	}
	in, _ := baseEngState()
	for i, op := range cases {
		_, err := groups.ApplyPatch(in, []groups.PatchOp{op})
		if !errors.Is(err, groups.ErrPatchNoTarget) {
			t.Errorf("case %d (%+v): err = %v, want ErrPatchNoTarget", i, op, err)
		}
	}
}

func TestApplyPatch_InvalidValueType(t *testing.T) {
	t.Parallel()
	in, _ := baseEngState()

	// displayName must be string.
	_, err := groups.ApplyPatch(in, []groups.PatchOp{{
		Op: "replace", Path: "displayName", Value: mustRaw(t, 42),
	}})
	if !errors.Is(err, groups.ErrPatchInvalidValue) {
		t.Errorf("displayName int: err = %v, want ErrPatchInvalidValue", err)
	}

	// members.value must be a UUID.
	_, err = groups.ApplyPatch(in, []groups.PatchOp{{
		Op:    "add",
		Path:  "members",
		Value: mustRaw(t, []any{map[string]any{"value": "not-a-uuid"}}),
	}})
	if !errors.Is(err, groups.ErrPatchInvalidValue) {
		t.Errorf("members non-uuid: err = %v, want ErrPatchInvalidValue", err)
	}
}

func TestApplyPatch_BatchAbortsOnFirstError(t *testing.T) {
	t.Parallel()
	in, _ := baseEngState()
	_, err := groups.ApplyPatch(in, []groups.PatchOp{
		{Op: "replace", Path: "displayName", Value: mustRaw(t, "Updated")},
		{Op: "remove", Path: "displayName"}, // not allowed
		{Op: "add", Path: "members", Value: mustRaw(t, []any{map[string]any{"value": "33333333-3333-3333-3333-333333333333"}})},
	})
	if !errors.Is(err, groups.ErrPatchNoTarget) {
		t.Fatalf("err = %v, want ErrPatchNoTarget", err)
	}
	if in.Properties[groups.PropertyKeyDisplayName] != "Engineering" {
		t.Errorf("input mutated mid-batch: %v", in.Properties[groups.PropertyKeyDisplayName])
	}
}

package users_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/danielpadua/oad/internal/scim/users"
)

func mustRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func basePropsActiveAlice() map[string]any {
	return map[string]any{
		users.PropertyKeyUserName:    "alice",
		users.PropertyKeyDisplayName: "Alice",
		users.PropertyKeyEmail:       "alice@example.com",
		users.PropertyKeyActive:      true,
	}
}

func TestApplyPatch_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	in := basePropsActiveAlice()
	out, err := users.ApplyPatch(in, []users.PatchOp{{
		Op: "replace", Path: "displayName", Value: mustRaw(t, "Alice Updated"),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if in[users.PropertyKeyDisplayName] != "Alice" {
		t.Errorf("input mutated: displayName = %v", in[users.PropertyKeyDisplayName])
	}
	if out[users.PropertyKeyDisplayName] != "Alice Updated" {
		t.Errorf("output displayName = %v, want Alice Updated", out[users.PropertyKeyDisplayName])
	}
}

func TestApplyPatch_ReplaceUserName(t *testing.T) {
	t.Parallel()
	out, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{{
		Op: "replace", Path: "userName", Value: mustRaw(t, "alice2"),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out[users.PropertyKeyUserName] != "alice2" {
		t.Errorf("userName = %v, want alice2", out[users.PropertyKeyUserName])
	}
}

func TestApplyPatch_ReplaceActive(t *testing.T) {
	t.Parallel()
	out, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{{
		Op: "replace", Path: "active", Value: mustRaw(t, false),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out[users.PropertyKeyActive] != false {
		t.Errorf("active = %v, want false", out[users.PropertyKeyActive])
	}
}

func TestApplyPatch_AddDisplayNameTreatedAsReplace(t *testing.T) {
	t.Parallel()
	out, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{{
		Op: "add", Path: "displayName", Value: mustRaw(t, "Alice A"),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out[users.PropertyKeyDisplayName] != "Alice A" {
		t.Errorf("displayName = %v, want Alice A", out[users.PropertyKeyDisplayName])
	}
}

func TestApplyPatch_ReplacePrimaryEmailValue(t *testing.T) {
	t.Parallel()
	out, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{{
		Op:    "replace",
		Path:  `emails[primary eq true].value`,
		Value: mustRaw(t, "new@example.com"),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out[users.PropertyKeyEmail] != "new@example.com" {
		t.Errorf("email = %v, want new@example.com", out[users.PropertyKeyEmail])
	}
}

func TestApplyPatch_AddEmailsArray(t *testing.T) {
	t.Parallel()
	out, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{{
		Op:    "add",
		Path:  "emails",
		Value: mustRaw(t, []any{map[string]any{"value": "second@example.com", "primary": true}}),
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out[users.PropertyKeyEmail] != "second@example.com" {
		t.Errorf("email = %v, want second@example.com", out[users.PropertyKeyEmail])
	}
}

func TestApplyPatch_RemovePrimaryEmail(t *testing.T) {
	t.Parallel()
	out, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{{
		Op:   "remove",
		Path: `emails[primary eq true]`,
	}})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if _, ok := out[users.PropertyKeyEmail]; ok {
		t.Errorf("email still present after remove: %v", out[users.PropertyKeyEmail])
	}
}

func TestApplyPatch_OperationsAppliedInOrder(t *testing.T) {
	t.Parallel()
	out, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{
		{Op: "replace", Path: "displayName", Value: mustRaw(t, "First")},
		{Op: "replace", Path: "displayName", Value: mustRaw(t, "Second")},
	})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out[users.PropertyKeyDisplayName] != "Second" {
		t.Errorf("displayName = %v, want Second", out[users.PropertyKeyDisplayName])
	}
}

func TestApplyPatch_NoTargetCases(t *testing.T) {
	t.Parallel()
	cases := []users.PatchOp{
		// Unsupported op.
		{Op: "move", Path: "displayName", Value: mustRaw(t, "x")},
		// remove on userName is not allowed.
		{Op: "remove", Path: "userName"},
		// replace with random sub-attr.
		{Op: "replace", Path: "name.formatted", Value: mustRaw(t, "x")},
		// add on userName not allowed (must use replace).
		{Op: "add", Path: "userName", Value: mustRaw(t, "x")},
		// Filter on displayName not allowed.
		{Op: "replace", Path: `displayName[primary eq true]`, Value: mustRaw(t, "x")},
		// Wrong filter shape on emails.
		{Op: "replace", Path: `emails[type eq "work"].value`, Value: mustRaw(t, "x")},
	}
	for i, op := range cases {
		_, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{op})
		if !errors.Is(err, users.ErrPatchNoTarget) {
			t.Errorf("case %d (%+v): err = %v, want ErrPatchNoTarget", i, op, err)
		}
	}
}

func TestApplyPatch_InvalidValueType(t *testing.T) {
	t.Parallel()
	_, err := users.ApplyPatch(basePropsActiveAlice(), []users.PatchOp{{
		Op: "replace", Path: "active", Value: mustRaw(t, "not-a-bool"),
	}})
	if !errors.Is(err, users.ErrPatchInvalidValue) {
		t.Errorf("err = %v, want ErrPatchInvalidValue", err)
	}
}

func TestApplyPatch_BatchAbortsOnFirstError(t *testing.T) {
	t.Parallel()
	in := basePropsActiveAlice()
	_, err := users.ApplyPatch(in, []users.PatchOp{
		{Op: "replace", Path: "displayName", Value: mustRaw(t, "Updated")},
		{Op: "remove", Path: "userName"}, // not allowed
		{Op: "replace", Path: "active", Value: mustRaw(t, false)},
	})
	if !errors.Is(err, users.ErrPatchNoTarget) {
		t.Fatalf("err = %v, want ErrPatchNoTarget", err)
	}
	// Original input must be untouched even when a partial pass started.
	if in[users.PropertyKeyDisplayName] != "Alice" {
		t.Errorf("input mutated mid-batch: %v", in[users.PropertyKeyDisplayName])
	}
}

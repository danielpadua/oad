package groups

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/scim/parser"
)

// PatchRequest is the JSON shape of a SCIM PATCH body (RFC 7644 §3.5.2.1).
type PatchRequest struct {
	Schemas    []string  `json:"schemas"`
	Operations []PatchOp `json:"Operations"`
}

// PatchOp is one entry in the Operations array. Value is left as
// json.RawMessage so each path can decode it into the type it expects
// (string, member object, array of member objects).
type PatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"`
}

// ErrPatchNoTarget indicates the PATCH op references a path that OAD
// does not expose for the requested operation. Translated to 400 with
// SCIM scimType=noTarget per RFC 7644 §3.12.
var ErrPatchNoTarget = errors.New("scim patch path is not supported for this operation")

// ErrPatchInvalidValue indicates the PATCH op's value field could not
// be decoded into the type expected by the targeted attribute.
// Translated to 400 with SCIM scimType=invalidValue.
var ErrPatchInvalidValue = errors.New("scim patch value is invalid for this path")

// PatchState carries the mutable inputs/outputs of ApplyPatch. Properties
// holds the entity.properties JSONB document; MemberIDs is the ordered
// member list (deduped on input). The two are returned together because
// SCIM PATCH may touch either or both — callers persist them atomically.
type PatchState struct {
	Properties map[string]any
	MemberIDs  []uuid.UUID
}

// ApplyPatch returns a fresh PatchState produced by applying ops in
// order to a copy of in. The original Properties map and MemberIDs
// slice are not mutated.
//
// Supported combinations (paths case-insensitive):
//
//	op=add     path=displayName              value=string
//	op=add     path=members                  value=Member | []Member
//	op=replace path=displayName              value=string
//	op=replace path=members                  value=[]Member       (full replace)
//	op=remove  path=members                                       (clear all)
//	op=remove  path=members[value eq "<id>"]                      (drop one)
//
// Anything else returns ErrPatchNoTarget.
func ApplyPatch(in PatchState, ops []PatchOp) (PatchState, error) {
	out := PatchState{
		Properties: make(map[string]any, len(in.Properties)+1),
		MemberIDs:  append([]uuid.UUID(nil), in.MemberIDs...),
	}
	maps.Copy(out.Properties, in.Properties)

	for i, op := range ops {
		if err := applyOne(&out, op); err != nil {
			return PatchState{}, fmt.Errorf("operation %d (%s %q): %w", i, op.Op, op.Path, err)
		}
	}
	return out, nil
}

func applyOne(state *PatchState, op PatchOp) error {
	path, err := parser.ParsePath(op.Path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPatchNoTarget, err)
	}

	switch strings.ToLower(op.Op) {
	case "add":
		return applyAdd(state, path, op.Value)
	case "replace":
		return applyReplace(state, path, op.Value)
	case "remove":
		return applyRemove(state, path)
	default:
		return fmt.Errorf("%w: unsupported op %q", ErrPatchNoTarget, op.Op)
	}
}

func applyAdd(state *PatchState, p parser.Path, raw json.RawMessage) error {
	if p.Attr == "" {
		return applyRootObject(raw, func(subPath parser.Path, subVal json.RawMessage) error {
			return applyAdd(state, subPath, subVal)
		})
	}
	if p.Filter != nil || p.SubAttr != "" {
		return ErrPatchNoTarget
	}
	switch p.Attr {
	case "displayname":
		return setStringProp(state.Properties, PropertyKeyDisplayName, raw)
	case "members":
		toAdd, err := decodeMembersValue(raw)
		if err != nil {
			return err
		}
		state.MemberIDs = mergeMembers(state.MemberIDs, toAdd)
		return nil
	}
	return ErrPatchNoTarget
}

func applyReplace(state *PatchState, p parser.Path, raw json.RawMessage) error {
	if p.Attr == "" {
		return applyRootObject(raw, func(subPath parser.Path, subVal json.RawMessage) error {
			return applyReplace(state, subPath, subVal)
		})
	}
	if p.Filter != nil || p.SubAttr != "" {
		return ErrPatchNoTarget
	}
	switch p.Attr {
	case "displayname":
		return setStringProp(state.Properties, PropertyKeyDisplayName, raw)
	case "members":
		next, err := decodeMembersValue(raw)
		if err != nil {
			return err
		}
		state.MemberIDs = dedupMembers(next)
		return nil
	}
	return ErrPatchNoTarget
}

func applyRemove(state *PatchState, p parser.Path) error {
	if p.Attr != "members" || p.SubAttr != "" {
		return ErrPatchNoTarget
	}
	// Clear all: `members` with no filter.
	if p.Filter == nil {
		state.MemberIDs = nil
		return nil
	}
	// Single drop: `members[value eq "<uuid>"]`.
	target, ok := memberValueFilter(p.Filter)
	if !ok {
		return ErrPatchNoTarget
	}
	state.MemberIDs = removeMember(state.MemberIDs, target)
	return nil
}

func applyRootObject(raw json.RawMessage, fn func(p parser.Path, v json.RawMessage) error) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("%w: expected object when path is empty", ErrPatchInvalidValue)
	}
	for k, v := range obj {
		subPath, err := parser.ParsePath(k)
		if err != nil {
			// RFC 7644: ignore unsupported attributes
			continue
		}
		if err := fn(subPath, v); err != nil {
			if errors.Is(err, ErrPatchNoTarget) {
				// RFC 7644 §3.5.2: "If the 'value' contains an attribute that is
				// not supported by the Service Provider or is read-only, the
				// Service Provider SHOULD ignore the attribute and continue"
				continue
			}
			return err
		}
	}
	return nil
}

// memberValueFilter returns the UUID encoded by a `value eq "<uuid>"`
// AtomExpr filter, or false if expr does not match that exact shape.
func memberValueFilter(expr parser.Expr) (uuid.UUID, bool) {
	atom, ok := expr.(*parser.AtomExpr)
	if !ok {
		return uuid.Nil, false
	}
	if atom.Attr != "value" || atom.Op != parser.OpEq {
		return uuid.Nil, false
	}
	s, ok := atom.Value.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// decodeMembersValue parses the SCIM members payload — either an array
// of Member objects or a single Member — and returns the list of UUIDs.
// Duplicates are tolerated here; callers dedupe at the merge/replace step.
func decodeMembersValue(raw json.RawMessage) ([]uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: members value is required", ErrPatchInvalidValue)
	}

	var arr []Member
	if err := json.Unmarshal(raw, &arr); err == nil {
		ids, perr := membersToUUIDs(arr)
		if perr != nil {
			return nil, perr
		}
		return ids, nil
	}

	var single Member
	if err := json.Unmarshal(raw, &single); err == nil && single.Value != "" {
		id, perr := uuid.Parse(single.Value)
		if perr != nil {
			return nil, fmt.Errorf("%w: member value %q is not a UUID", ErrPatchInvalidValue, single.Value)
		}
		return []uuid.UUID{id}, nil
	}

	return nil, fmt.Errorf("%w: members must be a Member object or an array of Member objects", ErrPatchInvalidValue)
}

func membersToUUIDs(in []Member) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(in))
	for _, m := range in {
		if m.Value == "" {
			return nil, fmt.Errorf("%w: member.value is required", ErrPatchInvalidValue)
		}
		id, err := uuid.Parse(m.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: member value %q is not a UUID", ErrPatchInvalidValue, m.Value)
		}
		out = append(out, id)
	}
	return out, nil
}

// mergeMembers appends toAdd onto base, skipping ids already present.
// Order is preserved: existing members keep their position, new members
// are appended in arrival order.
func mergeMembers(base, toAdd []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(base)+len(toAdd))
	for _, id := range base {
		seen[id] = struct{}{}
	}
	out := append([]uuid.UUID(nil), base...)
	for _, id := range toAdd {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// dedupMembers preserves first occurrence and drops subsequent duplicates.
func dedupMembers(in []uuid.UUID) []uuid.UUID {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(in))
	out := make([]uuid.UUID, 0, len(in))
	for _, id := range in {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// removeMember returns a copy of in with the first occurrence of target
// removed. Returns the input unchanged (as a copy) if target is absent —
// SCIM does not require remove on a missing member to fail.
func removeMember(in []uuid.UUID, target uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(in))
	dropped := false
	for _, id := range in {
		if !dropped && id == target {
			dropped = true
			continue
		}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func setStringProp(props map[string]any, key string, raw json.RawMessage) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("%w: expected string for %s", ErrPatchInvalidValue, key)
	}
	props[key] = s
	return nil
}

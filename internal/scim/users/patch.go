package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/danielpadua/oad/internal/scim/parser"
)

// PatchRequest is the JSON shape of a SCIM PATCH body
// (RFC 7644 §3.5.2.1).
type PatchRequest struct {
	Schemas    []string  `json:"schemas"`
	Operations []PatchOp `json:"Operations"`
}

// PatchOp is one entry in the Operations array. Value is left as
// json.RawMessage so each path can decode it into the type it expects
// (string, bool, complex object, etc.).
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

// ApplyPatch returns a fresh properties map produced by applying ops in
// order to a copy of in. The original map is not mutated. Operations
// are applied left-to-right; the first failure aborts the batch.
//
// Supported combinations (paths case-insensitive):
//
//	op=add     path=displayName | emails | active
//	op=replace path=displayName | userName | active
//	            path=emails[primary eq true].value
//	op=remove  path=emails[primary eq true]
//
// Anything else returns ErrPatchNoTarget.
func ApplyPatch(in map[string]any, ops []PatchOp) (map[string]any, error) {
	out := make(map[string]any, len(in)+1)
	maps.Copy(out, in)
	for i, op := range ops {
		if err := applyOne(out, op); err != nil {
			return nil, fmt.Errorf("operation %d (%s %q): %w", i, op.Op, op.Path, err)
		}
	}
	return out, nil
}

func applyOne(props map[string]any, op PatchOp) error {
	path, err := parser.ParsePath(op.Path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPatchNoTarget, err)
	}

	switch strings.ToLower(op.Op) {
	case "add":
		return applyAdd(props, path, op.Value)
	case "replace":
		return applyReplace(props, path, op.Value)
	case "remove":
		return applyRemove(props, path)
	default:
		return fmt.Errorf("%w: unsupported op %q", ErrPatchNoTarget, op.Op)
	}
}

func applyAdd(props map[string]any, p parser.Path, raw json.RawMessage) error {
	if p.Attr == "" {
		return applyRootObject(raw, func(subPath parser.Path, subVal json.RawMessage) error {
			return applyAdd(props, subPath, subVal)
		})
	}
	// Filters are not allowed on add for any User attribute in our subset.
	if p.Filter != nil || p.SubAttr != "" {
		return ErrPatchNoTarget
	}
	switch p.Attr {
	case "displayname":
		return setStringProp(props, PropertyKeyDisplayName, raw)
	case "active":
		return setBoolProp(props, PropertyKeyActive, raw)
	case "emails":
		return setEmailsFromValue(props, raw)
	}
	return ErrPatchNoTarget
}

func applyReplace(props map[string]any, p parser.Path, raw json.RawMessage) error {
	if p.Attr == "" {
		return applyRootObject(raw, func(subPath parser.Path, subVal json.RawMessage) error {
			return applyReplace(props, subPath, subVal)
		})
	}
	// emails[primary eq true].value — replace the persisted primary email.
	if p.Attr == "emails" && p.SubAttr == "value" && isPrimaryEqTrue(p.Filter) {
		return setStringProp(props, PropertyKeyEmail, raw)
	}
	if p.Filter != nil || p.SubAttr != "" {
		return ErrPatchNoTarget
	}
	switch p.Attr {
	case "displayname":
		return setStringProp(props, PropertyKeyDisplayName, raw)
	case "username":
		return setStringProp(props, PropertyKeyUserName, raw)
	case "active":
		return setBoolProp(props, PropertyKeyActive, raw)
	case "emails":
		return setEmailsFromValue(props, raw)
	}
	return ErrPatchNoTarget
}

func applyRemove(props map[string]any, p parser.Path) error {
	// Only emails[primary eq true] is supported for remove on Users.
	if p.Attr == "emails" && p.SubAttr == "" && isPrimaryEqTrue(p.Filter) {
		delete(props, PropertyKeyEmail)
		return nil
	}
	return ErrPatchNoTarget
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

// isPrimaryEqTrue reports whether expr is the AtomExpr `primary eq true`,
// which is the only filter shape we recognize for emails.
func isPrimaryEqTrue(expr parser.Expr) bool {
	atom, ok := expr.(*parser.AtomExpr)
	if !ok {
		return false
	}
	return atom.Attr == "primary" && atom.Op == parser.OpEq && atom.Value == true
}

func setStringProp(props map[string]any, key string, raw json.RawMessage) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("%w: expected string for %s", ErrPatchInvalidValue, key)
	}
	props[key] = s
	return nil
}

func setBoolProp(props map[string]any, key string, raw json.RawMessage) error {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return fmt.Errorf("%w: expected boolean for %s", ErrPatchInvalidValue, key)
	}
	props[key] = b
	return nil
}

// setEmailsFromValue decodes the SCIM `emails` payload (list of complex
// objects, or a single object) and stores the primary email on props.
// Mirrors pickPrimaryEmail's selection rule: first entry with primary=true,
// otherwise the first entry's value.
func setEmailsFromValue(props map[string]any, raw json.RawMessage) error {
	// Try array first.
	var arr []Email
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		if email := pickPrimaryEmail(arr); email != "" {
			props[PropertyKeyEmail] = email
			return nil
		}
		return fmt.Errorf("%w: no usable email in array", ErrPatchInvalidValue)
	}

	// Fallback: single complex object.
	var single Email
	if err := json.Unmarshal(raw, &single); err == nil && single.Value != "" {
		props[PropertyKeyEmail] = single.Value
		return nil
	}
	return fmt.Errorf("%w: emails must be an array of {value, primary} or a single object", ErrPatchInvalidValue)
}

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// assertStep applies the Expect block to one response and returns the
// list of failure messages. An empty slice means the step passed.
func assertStep(exp Expect, gotStatus int, body []byte) []string {
	var errs []string

	if exp.Status != 0 && exp.Status != gotStatus {
		errs = append(errs, fmt.Sprintf("status = %d, want %d (body=%s)", gotStatus, exp.Status, snippet(body)))
	}

	if exp.Body != nil {
		var got any
		if err := json.Unmarshal(body, &got); err != nil {
			errs = append(errs, fmt.Sprintf("decode body: %v", err))
		} else if e := matchPartial(exp.Body, got, ""); e != "" {
			errs = append(errs, "body mismatch: "+e)
		}
	}

	for _, sub := range exp.BodyContains {
		if !strings.Contains(string(body), sub) {
			errs = append(errs, fmt.Sprintf("body does not contain %q (body=%s)", sub, snippet(body)))
		}
	}
	return errs
}

// matchPartial recursively compares expected against got. Maps require
// every expected key to be present in got with a matching value but
// tolerate extra keys in got. Arrays compare element-wise: every
// element in expected must partial-match the same-indexed element in
// got. Primitives compare with deep equality.
//
// The path argument is used for error messages — it's the dotted path
// from the response root to the value being compared.
func matchPartial(expected, got any, path string) string {
	switch e := expected.(type) {
	case map[string]any:
		gm, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("at %q: want object, got %T", path, got)
		}
		for k, v := range e {
			subPath := joinPath(path, k)
			gv, present := gm[k]
			if !present {
				return fmt.Sprintf("at %q: key missing", subPath)
			}
			if msg := matchPartial(v, gv, subPath); msg != "" {
				return msg
			}
		}
		return ""
	case []any:
		ga, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("at %q: want array, got %T", path, got)
		}
		if len(e) > len(ga) {
			return fmt.Sprintf("at %q: array length = %d, want >= %d", path, len(ga), len(e))
		}
		for i, v := range e {
			subPath := fmt.Sprintf("%s[%d]", path, i)
			if msg := matchPartial(v, ga[i], subPath); msg != "" {
				return msg
			}
		}
		return ""
	default:
		// Primitives: JSON unmarshals numbers as float64. YAML may
		// produce int. Normalize both sides to float64 when one is
		// numeric so YAML-authored 201 matches JSON-decoded 201.
		if equalScalars(expected, got) {
			return ""
		}
		return fmt.Sprintf("at %q: got %v (%T), want %v (%T)", path, got, got, expected, expected)
	}
}

func equalScalars(a, b any) bool {
	if a == b {
		return true
	}
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		return af == bf
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

func joinPath(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return prefix + "." + segment
}

// snippet returns body trimmed to a readable length for error messages.
func snippet(body []byte) string {
	const maxLen = 200
	s := strings.TrimSpace(string(body))
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

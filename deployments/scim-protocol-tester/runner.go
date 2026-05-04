package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Runner executes a Scenario against a SCIM endpoint. The same Runner
// can be reused across scenarios; captures from one scenario do NOT
// leak into another.
type Runner struct {
	Client *http.Client
	// Stdout is where step status lines are written. nil falls back to
	// os.Stdout. Tests substitute a buffer.
	Stdout io.Writer
}

// Result is the outcome of a single Run.
type Result struct {
	ScenarioName string
	Steps        []StepResult
}

// StepResult records what happened on one step.
type StepResult struct {
	Name   string
	Status int
	Errors []string
}

// Failed reports whether any step in the result accumulated errors.
func (r Result) Failed() bool {
	for _, s := range r.Steps {
		if len(s.Errors) > 0 {
			return true
		}
	}
	return false
}

// Run executes scenario in order, stopping at the first step whose
// assertions fail. The returned Result contains one StepResult per
// step that was executed (so downstream callers can render a clean
// progress log).
func (r *Runner) Run(ctx context.Context, sc Scenario) (Result, error) {
	stdout := r.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}

	token, err := resolveToken(sc.Token)
	if err != nil {
		return Result{}, fmt.Errorf("resolve token: %w", err)
	}
	target := strings.TrimRight(sc.Target, "/")
	if target == "" {
		return Result{}, errors.New("scenario target is required")
	}

	res := Result{ScenarioName: sc.Name}
	captures := make(map[string]string)

	for i, step := range sc.Steps {
		sr := r.runStep(ctx, client, target, token, step, captures)
		res.Steps = append(res.Steps, sr)
		writeStepLine(stdout, i+1, sr)
		if len(sr.Errors) > 0 {
			return res, nil
		}
	}
	return res, nil
}

// runStep performs the per-step substitute → marshal → request → assert
// → capture pipeline. Any failure short-circuits and is reported in the
// returned StepResult; downstream loop body decides whether to continue.
func (r *Runner) runStep(
	ctx context.Context, client *http.Client,
	target, token string,
	step Step, captures map[string]string,
) StepResult {
	stepName := step.Name
	if stepName == "" {
		stepName = fmt.Sprintf("%s %s", step.Method, step.Path)
	}
	sr := StepResult{Name: stepName}

	path, err := substituteVars(step.Path, captures)
	if err != nil {
		sr.Errors = []string{fmt.Sprintf("path substitution: %v", err)}
		return sr
	}
	bodyBytes, err := marshalBodyWithVars(step.Body, captures)
	if err != nil {
		sr.Errors = []string{fmt.Sprintf("body marshal: %v", err)}
		return sr
	}
	req, err := http.NewRequestWithContext(ctx, step.Method, target+path, bytes.NewReader(bodyBytes))
	if err != nil {
		sr.Errors = []string{fmt.Sprintf("build request: %v", err)}
		return sr
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/scim+json")
	}
	req.Header.Set("Accept", "application/scim+json")
	for k, v := range step.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		sr.Errors = []string{fmt.Sprintf("transport: %v", err)}
		return sr
	}
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	sr.Status = resp.StatusCode
	sr.Errors = assertStep(step.Expect, resp.StatusCode, respBody)
	if len(sr.Errors) == 0 {
		if cerr := captureFromBody(step.Capture, respBody, captures); cerr != nil {
			sr.Errors = append(sr.Errors, fmt.Sprintf("capture: %v", cerr))
		}
	}
	return sr
}

// writeStepLine renders one status line per step. Errors are written
// indented under the step header for readability.
func writeStepLine(w io.Writer, num int, sr StepResult) {
	if len(sr.Errors) > 0 {
		_, _ = fmt.Fprintf(w, "  ✘ step %d %q — status=%d\n", num, sr.Name, sr.Status)
		for _, e := range sr.Errors {
			_, _ = fmt.Fprintf(w, "      %s\n", e)
		}
		return
	}
	_, _ = fmt.Fprintf(w, "  ✓ step %d %q — status=%d\n", num, sr.Name, sr.Status)
}

// resolveToken expands `env:VAR` to the value of $VAR. Plain literals
// pass through unchanged. An empty input returns "".
func resolveToken(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	if rest, ok := strings.CutPrefix(token, "env:"); ok {
		v := os.Getenv(rest)
		if v == "" {
			return "", fmt.Errorf("env var %q is empty", rest)
		}
		return v, nil
	}
	return token, nil
}

// substituteVars replaces every `{name}` token in s with the captured
// value of `name`. An unknown name produces an error so that scenarios
// fail loudly rather than emitting `{undefined}` to the network.
func substituteVars(s string, vars map[string]string) (string, error) {
	if !strings.ContainsRune(s, '{') {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c != '{' {
			b.WriteByte(c)
			i++
			continue
		}
		end := strings.IndexByte(s[i:], '}')
		if end < 0 {
			return "", fmt.Errorf("unterminated `{` at offset %d", i)
		}
		name := s[i+1 : i+end]
		val, ok := vars[name]
		if !ok {
			return "", fmt.Errorf("undefined variable %q", name)
		}
		b.WriteString(val)
		i += end + 1
	}
	return b.String(), nil
}

// marshalBodyWithVars walks body and JSON-marshals it after replacing
// `{var}` placeholders inside any string leaf. A nil body yields an
// empty byte slice (so GET/DELETE produce no Content-Length).
func marshalBodyWithVars(body any, vars map[string]string) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	subbed, err := walkSubstitute(body, vars)
	if err != nil {
		return nil, err
	}
	return json.Marshal(subbed)
}

// walkSubstitute recursively traverses v applying substituteVars to
// every string leaf. Map keys are preserved verbatim. Non-string,
// non-map, non-slice values are returned unchanged. YAML decoders
// produce `map[string]any` for mappings and `[]any` for sequences,
// which is what we expect here.
func walkSubstitute(v any, vars map[string]string) (any, error) {
	switch x := v.(type) {
	case string:
		return substituteVars(x, vars)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			next, err := walkSubstitute(val, vars)
			if err != nil {
				return nil, err
			}
			out[k] = next
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			next, err := walkSubstitute(val, vars)
			if err != nil {
				return nil, err
			}
			out[i] = next
		}
		return out, nil
	default:
		return v, nil
	}
}

// captureFromBody extracts each path in capture from raw and writes the
// stringified value into vars. Non-string leaves (numbers, bools) are
// converted with fmt.Sprintf("%v", ...). Missing paths produce an error
// because a captured value referenced by a downstream step would later
// fail with a confusing "undefined variable" error.
func captureFromBody(capture map[string]string, raw []byte, vars map[string]string) error {
	if len(capture) == 0 {
		return nil
	}
	if len(raw) == 0 {
		return fmt.Errorf("response body is empty, cannot capture %d value(s)", len(capture))
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	for name, path := range capture {
		val, err := jsonPath(parsed, path)
		if err != nil {
			return fmt.Errorf("capture %q at path %q: %w", name, path, err)
		}
		vars[name] = stringify(val)
	}
	return nil
}

// jsonPath walks a dotted path inside a decoded JSON value. Numeric
// segments index into arrays; everything else is treated as a map key.
// "" returns root.
func jsonPath(root any, path string) (any, error) {
	if path == "" {
		return root, nil
	}
	cur := root
	for seg := range strings.SplitSeq(path, ".") {
		switch x := cur.(type) {
		case map[string]any:
			next, ok := x[seg]
			if !ok {
				return nil, fmt.Errorf("key %q not found", seg)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return nil, fmt.Errorf("segment %q is not an array index", seg)
			}
			if idx < 0 || idx >= len(x) {
				return nil, fmt.Errorf("array index %d out of range (len=%d)", idx, len(x))
			}
			cur = x[idx]
		default:
			return nil, fmt.Errorf("cannot traverse into %T at segment %q", cur, seg)
		}
	}
	return cur, nil
}

func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

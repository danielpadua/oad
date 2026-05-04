// Package main implements scim-protocol-tester — a YAML-scenario-driven
// raw SCIM 2.0 client used for protocol-edge tests against the OAD SCIM
// surface. It is NOT part of `make dev`. The tool issues HTTP requests
// exactly as written, asserts response status + a partial JSON body
// match, and supports value capture across steps so a downstream PATCH
// or GET can reference a previously-created resource by id.
//
// Schema source of truth: docs/design/scim-ingest.md §12.2.
package main

// Scenario is a single YAML file loaded from disk. Target and Token
// apply to every step in Steps; Step-level overrides are intentionally
// not supported in v1 (every scenario hits one server with one tenant
// token, by construction).
type Scenario struct {
	// Name is shown in the runner output. Defaults to the YAML file
	// path when omitted.
	Name string `yaml:"name,omitempty"`

	// Target is the base URL up to and including `/scim/v2`. Each Step
	// path is appended to this base. Trailing slash is tolerated.
	Target string `yaml:"target"`

	// Token is the Bearer token sent on every request. The literal
	// prefix `env:` resolves the rest as an environment variable name
	// at runtime so secrets do not live in YAML — `env:OAD_SCIM_TOKEN`
	// reads $OAD_SCIM_TOKEN.
	Token string `yaml:"token,omitempty"`

	// Steps is the ordered list of HTTP operations to issue.
	Steps []Step `yaml:"scenario"`
}

// Step is one HTTP request in a scenario. All fields except Method and
// Path are optional. Path supports `{var}` placeholders that are
// substituted from values previously captured by upstream steps. Body,
// when present as a YAML mapping or sequence, is marshaled as JSON
// before being sent.
type Step struct {
	// Name is shown in the runner output. Defaults to "<METHOD> <path>".
	Name string `yaml:"name,omitempty"`

	// Method is the HTTP verb (GET, POST, PUT, PATCH, DELETE).
	Method string `yaml:"method"`

	// Path is appended to Scenario.Target. May contain `{var}`
	// placeholders that resolve against captured values.
	Path string `yaml:"path"`

	// Body is the JSON request body. Decoded as `any` so YAML mappings
	// and sequences round-trip cleanly to JSON.
	Body any `yaml:"body,omitempty"`

	// Headers are extra request headers. Authorization and
	// Content-Type are set by the runner and need not be repeated.
	Headers map[string]string `yaml:"headers,omitempty"`

	// Expect is the response assertion. A zero Expect (no fields set)
	// is permitted but useless — every step should at least assert a
	// status to be a meaningful test.
	Expect Expect `yaml:"expect"`

	// Capture extracts values from the response JSON for use in
	// subsequent steps. Keys are variable names; values are dotted JSON
	// paths (`id`, `Resources.0.id`, `meta.location`).
	Capture map[string]string `yaml:"capture,omitempty"`
}

// Expect describes the assertion applied to a step's response.
type Expect struct {
	// Status is the expected HTTP status code. 0 means "do not check"
	// — useful when a step's value is captured but the status is
	// transport-irrelevant.
	Status int `yaml:"status,omitempty"`

	// Body is a partial-match JSON document. Any key present here must
	// be present in the response body with the same value (deep equal
	// for primitives; recursive partial-match for objects; element-wise
	// partial match for arrays — every element of the expected array
	// must partial-match the response array element at the same index).
	Body any `yaml:"body,omitempty"`

	// BodyContains is a list of substrings that must each appear in
	// the raw response body. Useful for asserting SCIM scimType values
	// when the full response shape is not stable.
	BodyContains []string `yaml:"body_contains,omitempty"`
}

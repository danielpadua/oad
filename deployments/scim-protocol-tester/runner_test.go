package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeServer captures every received request so tests can assert on
// what the runner actually sent over the wire.
type fakeServer struct {
	t        *testing.T
	server   *httptest.Server
	requests []recordedRequest
	handler  func(req recordedRequest) (status int, body any)
}

type recordedRequest struct {
	Method  string
	Path    string
	Body    any
	Headers http.Header
}

func newFakeServer(t *testing.T, handler func(req recordedRequest) (int, any)) *fakeServer {
	t.Helper()
	fs := &fakeServer{t: t, handler: handler}
	fs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		var decoded any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Errorf("server: decode body: %v (raw=%s)", err, string(raw))
			}
		}
		req := recordedRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Body:    decoded,
			Headers: r.Header.Clone(),
		}
		fs.requests = append(fs.requests, req)
		status, body := fs.handler(req)
		w.Header().Set("Content-Type", "application/scim+json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
	t.Cleanup(fs.server.Close)
	return fs
}

func TestRunner_Roundtrip_CapturesAndSubstitutes(t *testing.T) {
	t.Parallel()

	const userID = "11111111-1111-1111-1111-111111111111"
	fs := newFakeServer(t, func(req recordedRequest) (int, any) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/scim/v2/Users":
			return http.StatusCreated, map[string]any{
				"id":          userID,
				"userName":    "alice",
				"displayName": "Alice",
			}
		case req.Method == http.MethodGet && req.Path == "/scim/v2/Users/"+userID:
			return http.StatusOK, map[string]any{
				"id":          userID,
				"userName":    "alice",
				"displayName": "Alice",
			}
		case req.Method == http.MethodDelete && req.Path == "/scim/v2/Users/"+userID:
			return http.StatusNoContent, nil
		}
		return http.StatusNotFound, map[string]any{"error": "no route"}
	})

	sc := Scenario{
		Name:   "user roundtrip",
		Target: fs.server.URL + "/scim/v2",
		Token:  "literal-token",
		Steps: []Step{
			{
				Name:    "create",
				Method:  http.MethodPost,
				Path:    "/Users",
				Body:    map[string]any{"userName": "alice"},
				Expect:  Expect{Status: 201, Body: map[string]any{"userName": "alice"}},
				Capture: map[string]string{"uid": "id"},
			},
			{
				Method: http.MethodGet,
				Path:   "/Users/{uid}",
				Expect: Expect{Status: 200},
			},
			{
				Method: http.MethodDelete,
				Path:   "/Users/{uid}",
				Expect: Expect{Status: 204},
			},
		},
	}

	var stdout bytes.Buffer
	r := &Runner{Stdout: &stdout}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Failed() {
		t.Fatalf("scenario failed: %#v\n%s", res, stdout.String())
	}
	if len(fs.requests) != 3 {
		t.Fatalf("server saw %d requests, want 3", len(fs.requests))
	}
	if got := fs.requests[1].Path; got != "/scim/v2/Users/"+userID {
		t.Errorf("captured path substitution = %q, want /scim/v2/Users/%s", got, userID)
	}
	if got := fs.requests[0].Headers.Get("Authorization"); got != "Bearer literal-token" {
		t.Errorf("auth header = %q, want Bearer literal-token", got)
	}
	if got := fs.requests[0].Headers.Get("Content-Type"); got != "application/scim+json" {
		t.Errorf("content-type = %q, want application/scim+json", got)
	}
}

func TestRunner_Substitution_InsideBody(t *testing.T) {
	t.Parallel()

	const groupID = "22222222-2222-2222-2222-222222222222"
	const userID = "33333333-3333-3333-3333-333333333333"

	fs := newFakeServer(t, func(req recordedRequest) (int, any) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/scim/v2/Groups":
			return http.StatusCreated, map[string]any{"id": groupID, "displayName": "Eng"}
		case req.Method == http.MethodPatch && req.Path == "/scim/v2/Groups/"+groupID:
			body, _ := req.Body.(map[string]any)
			ops, _ := body["Operations"].([]any)
			if len(ops) != 1 {
				t.Errorf("Operations len = %d, want 1", len(ops))
			}
			return http.StatusOK, map[string]any{"id": groupID, "displayName": "Eng"}
		}
		return http.StatusNotFound, nil
	})

	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Token:  "tok",
		Steps: []Step{
			{
				Method:  http.MethodPost,
				Path:    "/Groups",
				Body:    map[string]any{"displayName": "Eng"},
				Expect:  Expect{Status: 201},
				Capture: map[string]string{"gid": "id"},
			},
			{
				Method: http.MethodPatch,
				Path:   "/Groups/{gid}",
				Body: map[string]any{
					"Operations": []any{
						map[string]any{
							"op":    "add",
							"path":  "members",
							"value": []any{map[string]any{"value": userID}},
						},
					},
				},
				Expect: Expect{Status: 200},
			},
		},
	}

	r := &Runner{Stdout: io.Discard}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Failed() {
		t.Fatalf("scenario failed: %+v", res)
	}
	if got := fs.requests[1].Path; got != "/scim/v2/Groups/"+groupID {
		t.Errorf("PATCH path = %q, want /scim/v2/Groups/%s", got, groupID)
	}
}

func TestRunner_StatusMismatchAborts(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t, func(_ recordedRequest) (int, any) {
		return http.StatusConflict, map[string]any{"scimType": "uniqueness"}
	})

	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Steps: []Step{
			{Method: http.MethodPost, Path: "/Users", Body: map[string]any{"userName": "x"}, Expect: Expect{Status: 201}},
			{Method: http.MethodGet, Path: "/Users/x", Expect: Expect{Status: 200}},
		},
	}
	r := &Runner{Stdout: io.Discard}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Failed() {
		t.Fatalf("expected failure, got success")
	}
	if len(res.Steps) != 1 {
		t.Errorf("steps run = %d, want 1 (abort on first failure)", len(res.Steps))
	}
}

func TestRunner_BodyContainsAssertion(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t, func(_ recordedRequest) (int, any) {
		return http.StatusBadRequest, map[string]any{
			"schemas":  []any{"urn:ietf:params:scim:api:messages:2.0:Error"},
			"scimType": "noTarget",
			"detail":   "patch failed",
		}
	})

	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Steps: []Step{
			{
				Method: http.MethodPatch,
				Path:   "/Users/x",
				Body:   map[string]any{"Operations": []any{}},
				Expect: Expect{Status: 400, BodyContains: []string{"noTarget"}},
			},
		},
	}
	r := &Runner{Stdout: io.Discard}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Failed() {
		t.Fatalf("scenario failed: %+v", res)
	}
}

func TestRunner_UndefinedVarFails(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t, func(_ recordedRequest) (int, any) {
		return http.StatusOK, nil
	})
	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Steps: []Step{
			{Method: http.MethodGet, Path: "/Users/{undefined}", Expect: Expect{Status: 200}},
		},
	}
	r := &Runner{Stdout: io.Discard}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Failed() {
		t.Fatalf("expected failure for undefined variable")
	}
	if !strings.Contains(res.Steps[0].Errors[0], "undefined variable") {
		t.Errorf("error = %q, want substring 'undefined variable'", res.Steps[0].Errors[0])
	}
	if len(fs.requests) != 0 {
		t.Errorf("server received requests despite substitution failure: %d", len(fs.requests))
	}
}

func TestRunner_TokenFromEnv(t *testing.T) {
	t.Setenv("OAD_SCIM_TOKEN_TEST", "secret-from-env")

	fs := newFakeServer(t, func(_ recordedRequest) (int, any) {
		return http.StatusOK, nil
	})
	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Token:  "env:OAD_SCIM_TOKEN_TEST",
		Steps:  []Step{{Method: http.MethodGet, Path: "/Users", Expect: Expect{Status: 200}}},
	}
	r := &Runner{Stdout: io.Discard}
	if _, err := r.Run(t.Context(), sc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := fs.requests[0].Headers.Get("Authorization"); got != "Bearer secret-from-env" {
		t.Errorf("auth header = %q, want Bearer secret-from-env", got)
	}
}

func TestRunner_TokenFromEnv_Missing(t *testing.T) {
	t.Parallel()

	sc := Scenario{
		Target: "http://example.invalid/scim/v2",
		Token:  "env:DEFINITELY_NOT_SET_PROTOCOL_TESTER",
		Steps:  []Step{{Method: http.MethodGet, Path: "/Users", Expect: Expect{Status: 200}}},
	}
	r := &Runner{Stdout: io.Discard}
	_, err := r.Run(t.Context(), sc)
	if err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Errorf("err = %v, want 'is empty'", err)
	}
}

func TestRunner_PartialBodyMatch_NestedAndArrays(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t, func(_ recordedRequest) (int, any) {
		return http.StatusOK, map[string]any{
			"id":          "abc",
			"displayName": "Eng",
			"members": []any{
				map[string]any{"value": "u1", "display": "User One"},
				map[string]any{"value": "u2", "display": "User Two"},
			},
			"meta": map[string]any{"resourceType": "Group"},
		}
	})

	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Steps: []Step{
			{
				Method: http.MethodGet,
				Path:   "/Groups/abc",
				Expect: Expect{
					Status: 200,
					Body: map[string]any{
						"displayName": "Eng",
						"members": []any{
							map[string]any{"value": "u1"},
							map[string]any{"value": "u2"},
						},
						"meta": map[string]any{"resourceType": "Group"},
					},
				},
			},
		},
	}
	r := &Runner{Stdout: io.Discard}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Failed() {
		t.Fatalf("scenario failed: %+v", res)
	}
}

func TestRunner_PartialBodyMatch_DetectsMismatch(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t, func(_ recordedRequest) (int, any) {
		return http.StatusOK, map[string]any{"displayName": "Eng"}
	})
	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Steps: []Step{
			{
				Method: http.MethodGet,
				Path:   "/Groups/abc",
				Expect: Expect{Body: map[string]any{"displayName": "Marketing"}},
			},
		},
	}
	r := &Runner{Stdout: io.Discard}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Failed() {
		t.Fatalf("expected mismatch failure")
	}
}

func TestRunner_CaptureFromArrayElement(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t, func(req recordedRequest) (int, any) {
		if req.Method == http.MethodGet && req.Path == "/scim/v2/Users" {
			return http.StatusOK, map[string]any{
				"Resources": []any{
					map[string]any{"id": "first-id", "userName": "alice"},
				},
				"totalResults": 1,
			}
		}
		if req.Path == "/scim/v2/Users/first-id" {
			return http.StatusOK, map[string]any{"id": "first-id"}
		}
		return http.StatusNotFound, nil
	})

	sc := Scenario{
		Target: fs.server.URL + "/scim/v2",
		Steps: []Step{
			{
				Method:  http.MethodGet,
				Path:    "/Users",
				Expect:  Expect{Status: 200},
				Capture: map[string]string{"firstID": "Resources.0.id"},
			},
			{Method: http.MethodGet, Path: "/Users/{firstID}", Expect: Expect{Status: 200}},
		},
	}
	r := &Runner{Stdout: io.Discard}
	res, err := r.Run(t.Context(), sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Failed() {
		t.Fatalf("scenario failed: %+v", res)
	}
	if got := fs.requests[1].Path; got != "/scim/v2/Users/first-id" {
		t.Errorf("path = %q, want /scim/v2/Users/first-id", got)
	}
}

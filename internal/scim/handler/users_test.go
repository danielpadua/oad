package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	scimauth "github.com/danielpadua/oad/internal/scim/auth"
	"github.com/danielpadua/oad/internal/scim/handler"
	"github.com/danielpadua/oad/internal/scim/users"
)

// mockUsersService implements handler.UsersService with hand-tuned outputs.
// Unused methods of the interface return ErrNotImplemented to surface
// accidental usage in tests that didn't set them up.
type mockUsersService struct {
	createFn  func(ctx context.Context, providerName string, u users.User) (*users.User, error)
	getFn     func(ctx context.Context, providerName string, id uuid.UUID) (*users.User, error)
	listFn    func(ctx context.Context, providerName, filterStr string, offset, limit int) (*users.ListResult, error)
	replaceFn func(ctx context.Context, providerName string, id uuid.UUID, u users.User) (*users.User, error)
	patchFn   func(ctx context.Context, providerName string, id uuid.UUID, ops []users.PatchOp) (*users.User, error)
	deleteFn  func(ctx context.Context, providerName string, id uuid.UUID) error
}

var errNotImplemented = errors.New("mock: not implemented")

func (m *mockUsersService) Create(ctx context.Context, providerName string, u users.User) (*users.User, error) {
	if m.createFn == nil {
		return nil, errNotImplemented
	}
	return m.createFn(ctx, providerName, u)
}

func (m *mockUsersService) GetByEntityID(ctx context.Context, providerName string, id uuid.UUID) (*users.User, error) {
	if m.getFn == nil {
		return nil, errNotImplemented
	}
	return m.getFn(ctx, providerName, id)
}

func (m *mockUsersService) List(ctx context.Context, providerName, filterStr string, offset, limit int) (*users.ListResult, error) {
	if m.listFn == nil {
		return nil, errNotImplemented
	}
	return m.listFn(ctx, providerName, filterStr, offset, limit)
}

func (m *mockUsersService) Replace(ctx context.Context, providerName string, id uuid.UUID, u users.User) (*users.User, error) {
	if m.replaceFn == nil {
		return nil, errNotImplemented
	}
	return m.replaceFn(ctx, providerName, id, u)
}

func (m *mockUsersService) Patch(ctx context.Context, providerName string, id uuid.UUID, ops []users.PatchOp) (*users.User, error) {
	if m.patchFn == nil {
		return nil, errNotImplemented
	}
	return m.patchFn(ctx, providerName, id, ops)
}

func (m *mockUsersService) Delete(ctx context.Context, providerName string, id uuid.UUID) error {
	if m.deleteFn == nil {
		return errNotImplemented
	}
	return m.deleteFn(ctx, providerName, id)
}

// newAuthRouter wires the UsersHandler with a registered token, returning a
// chi router that exercises the full auth middleware chain.
func newAuthRouter(t *testing.T, svc handler.UsersService) (router chi.Router, token string) {
	t.Helper()
	registry := scimauth.NewRegistry()
	token = "test-token"
	if err := registry.Register("keycloak", token); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}
	router = chi.NewRouter()
	handler.Mount(router, registry, handler.Handlers{Users: handler.NewUsersHandler(svc)})
	return router, token
}

func sampleStored(id uuid.UUID) *users.User {
	t := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	stored := users.StoredUser{
		EntityID:        id,
		Properties:      map[string]any{"userName": "alice", "active": true},
		ExternalSubject: "alice-kc",
		CreatedAt:       t,
		UpdatedAt:       t,
	}
	out := users.FromStored(stored)
	return &out
}

func TestUsers_Create_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		createFn: func(_ context.Context, providerName string, u users.User) (*users.User, error) {
			if providerName != "keycloak" {
				t.Errorf("providerName = %q, want keycloak", providerName)
			}
			if u.UserName != "alice" {
				t.Errorf("UserName = %q, want alice", u.UserName)
			}
			return sampleStored(id), nil
		},
	}
	r, token := newAuthRouter(t, svc)

	body := strings.NewReader(`{"userName":"alice","externalId":"alice-kc","active":true}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Users", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/scim+json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got == "" || !strings.Contains(got, id.String()) {
		t.Errorf("Location header = %q, want to contain %q", got, id.String())
	}
	if got := rr.Header().Get("ETag"); !strings.HasPrefix(got, `W/"`) {
		t.Errorf("ETag = %q, want weak prefix", got)
	}

	var out users.User
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != id.String() {
		t.Errorf("response id = %q, want %q", out.ID, id.String())
	}
}

func TestUsers_Create_RejectedWithoutAuth(t *testing.T) {
	t.Parallel()

	svc := &mockUsersService{}
	r, _ := newAuthRouter(t, svc)

	body := strings.NewReader(`{"userName":"alice","externalId":"alice-kc"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Users", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestUsers_Create_ErrorMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		err        error
		body       string
		wantStatus int
		wantSCIM   string
	}{
		{"userName missing", users.ErrUserNameRequired, `{"externalId":"x"}`, http.StatusBadRequest, "invalidValue"},
		{"externalId missing", users.ErrExternalIDRequired, `{"userName":"alice"}`, http.StatusBadRequest, "invalidValue"},
		{"already exists", users.ErrAlreadyExists, `{"userName":"alice","externalId":"x"}`, http.StatusConflict, "uniqueness"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &mockUsersService{
				createFn: func(context.Context, string, users.User) (*users.User, error) {
					return nil, tc.err
				},
			}
			r, token := newAuthRouter(t, svc)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Users", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d; body=%s", rr.Code, tc.wantStatus, rr.Body.String())
			}
			var body map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body["scimType"] != tc.wantSCIM {
				t.Errorf("scimType = %v, want %q", body["scimType"], tc.wantSCIM)
			}
		})
	}
}

func TestUsers_Create_MalformedJSON(t *testing.T) {
	t.Parallel()

	svc := &mockUsersService{}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Users", strings.NewReader(`not json`))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestUsers_GetByID_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		getFn: func(_ context.Context, providerName string, gotID uuid.UUID) (*users.User, error) {
			if providerName != "keycloak" {
				t.Errorf("providerName = %q, want keycloak", providerName)
			}
			if gotID != id {
				t.Errorf("id = %v, want %v", gotID, id)
			}
			return sampleStored(id), nil
		},
	}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users/"+id.String(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("ETag") == "" {
		t.Errorf("missing ETag header")
	}
}

func TestUsers_GetByID_BadUUID(t *testing.T) {
	t.Parallel()

	svc := &mockUsersService{}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users/not-a-uuid", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestUsers_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		getFn: func(context.Context, string, uuid.UUID) (*users.User, error) {
			return nil, users.ErrNotFound
		},
	}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users/"+id.String(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestUsers_Delete_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	called := false
	svc := &mockUsersService{
		deleteFn: func(_ context.Context, providerName string, gotID uuid.UUID) error {
			called = true
			if providerName != "keycloak" {
				t.Errorf("providerName = %q, want keycloak", providerName)
			}
			if gotID != id {
				t.Errorf("id = %v, want %v", gotID, id)
			}
			return nil
		},
	}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/scim/v2/Users/"+id.String(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if !called {
		t.Errorf("service.Delete was not called")
	}
	body, _ := io.ReadAll(rr.Body)
	if len(body) != 0 {
		t.Errorf("expected empty body, got %q", body)
	}
}

func TestUsers_List_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		listFn: func(_ context.Context, providerName, filterStr string, offset, limit int) (*users.ListResult, error) {
			if providerName != "keycloak" {
				t.Errorf("providerName = %q, want keycloak", providerName)
			}
			if filterStr != `userName eq "alice"` {
				t.Errorf("filterStr = %q, want filter", filterStr)
			}
			if offset != 0 || limit != 50 {
				t.Errorf("offset/limit = %d/%d, want 0/50", offset, limit)
			}
			return &users.ListResult{Users: []*users.User{sampleStored(id)}, Total: 1}, nil
		},
	}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		`/scim/v2/Users?filter=userName+eq+%22alice%22`, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["totalResults"].(float64) != 1 {
		t.Errorf("totalResults = %v, want 1", body["totalResults"])
	}
	if body["startIndex"].(float64) != 1 {
		t.Errorf("startIndex = %v, want 1", body["startIndex"])
	}
	if body["itemsPerPage"].(float64) != 1 {
		t.Errorf("itemsPerPage = %v, want 1", body["itemsPerPage"])
	}
}

func TestUsers_List_PaginationClamps(t *testing.T) {
	t.Parallel()

	var gotOffset, gotLimit int
	svc := &mockUsersService{
		listFn: func(_ context.Context, _, _ string, offset, limit int) (*users.ListResult, error) {
			gotOffset, gotLimit = offset, limit
			return &users.ListResult{Total: 0}, nil
		},
	}
	r, token := newAuthRouter(t, svc)

	// startIndex=10 → offset=9; count=500 → clamp to 200.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		`/scim/v2/Users?startIndex=10&count=500`, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(httptest.NewRecorder(), req)
	if gotOffset != 9 || gotLimit != 200 {
		t.Errorf("offset/limit = %d/%d, want 9/200", gotOffset, gotLimit)
	}

	// Negative startIndex → clamp to 1 → offset=0; missing count → default 50.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		`/scim/v2/Users?startIndex=-5`, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(httptest.NewRecorder(), req)
	if gotOffset != 0 || gotLimit != 50 {
		t.Errorf("offset/limit = %d/%d, want 0/50", gotOffset, gotLimit)
	}
}

func TestUsers_List_InvalidFilter(t *testing.T) {
	t.Parallel()

	svc := &mockUsersService{
		listFn: func(context.Context, string, string, int, int) (*users.ListResult, error) {
			return nil, users.ErrInvalidFilter
		},
	}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		`/scim/v2/Users?filter=garbage`, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var body map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["scimType"] != "invalidFilter" {
		t.Errorf("scimType = %v, want invalidFilter", body["scimType"])
	}
}

func TestUsers_Replace_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		replaceFn: func(_ context.Context, providerName string, gotID uuid.UUID, in users.User) (*users.User, error) {
			if providerName != "keycloak" {
				t.Errorf("providerName = %q, want keycloak", providerName)
			}
			if gotID != id {
				t.Errorf("id = %v, want %v", gotID, id)
			}
			if in.UserName != "alice2" {
				t.Errorf("UserName = %q, want alice2", in.UserName)
			}
			return sampleStored(id), nil
		},
	}
	r, token := newAuthRouter(t, svc)

	body := strings.NewReader(`{"userName":"alice2","active":true}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/scim/v2/Users/"+id.String(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("ETag") == "" {
		t.Errorf("missing ETag header on PUT response")
	}
}

func TestUsers_Replace_BadUUID(t *testing.T) {
	t.Parallel()

	svc := &mockUsersService{}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/scim/v2/Users/not-a-uuid", strings.NewReader(`{"userName":"x"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestUsers_Replace_NotFound(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		replaceFn: func(context.Context, string, uuid.UUID, users.User) (*users.User, error) {
			return nil, users.ErrNotFound
		},
	}
	r, token := newAuthRouter(t, svc)

	body := strings.NewReader(`{"userName":"alice"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/scim/v2/Users/"+id.String(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestUsers_Patch_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	var gotOps []users.PatchOp
	svc := &mockUsersService{
		patchFn: func(_ context.Context, providerName string, gotID uuid.UUID, ops []users.PatchOp) (*users.User, error) {
			if providerName != "keycloak" {
				t.Errorf("provider = %q, want keycloak", providerName)
			}
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			gotOps = ops
			return sampleStored(id), nil
		},
	}
	r, token := newAuthRouter(t, svc)

	body := strings.NewReader(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"displayName","value":"Alice Updated"},{"op":"replace","path":"active","value":false}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Users/"+id.String(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/scim+json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if len(gotOps) != 2 {
		t.Fatalf("Operations forwarded = %d, want 2", len(gotOps))
	}
	if gotOps[0].Op != "replace" || gotOps[0].Path != "displayName" {
		t.Errorf("op[0] = %+v", gotOps[0])
	}
	if got := rr.Header().Get("ETag"); !strings.HasPrefix(got, `W/"`) {
		t.Errorf("ETag = %q, want weak prefix", got)
	}
}

func TestUsers_Patch_NoTarget(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		patchFn: func(context.Context, string, uuid.UUID, []users.PatchOp) (*users.User, error) {
			return nil, users.ErrPatchNoTarget
		},
	}
	r, token := newAuthRouter(t, svc)

	body := strings.NewReader(`{"Operations":[{"op":"remove","path":"userName"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Users/"+id.String(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"noTarget"`) {
		t.Errorf("body missing scimType=noTarget: %s", rr.Body.String())
	}
}

func TestUsers_Patch_InvalidValue(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		patchFn: func(context.Context, string, uuid.UUID, []users.PatchOp) (*users.User, error) {
			return nil, users.ErrPatchInvalidValue
		},
	}
	r, token := newAuthRouter(t, svc)

	body := strings.NewReader(`{"Operations":[{"op":"replace","path":"active","value":"not-a-bool"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Users/"+id.String(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"invalidValue"`) {
		t.Errorf("body missing scimType=invalidValue: %s", rr.Body.String())
	}
}

func TestUsers_Patch_BadUUID(t *testing.T) {
	t.Parallel()

	r, token := newAuthRouter(t, &mockUsersService{})

	body := strings.NewReader(`{"Operations":[]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Users/not-a-uuid", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestUsers_Delete_NotFound(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUsersService{
		deleteFn: func(context.Context, string, uuid.UUID) error {
			return users.ErrNotFound
		},
	}
	r, token := newAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/scim/v2/Users/"+id.String(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

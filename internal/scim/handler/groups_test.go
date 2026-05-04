package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	scimauth "github.com/danielpadua/oad/internal/scim/auth"
	"github.com/danielpadua/oad/internal/scim/groups"
	"github.com/danielpadua/oad/internal/scim/handler"
)

type mockGroupsService struct {
	createFn  func(ctx context.Context, providerName string, g groups.Group) (*groups.Group, error)
	getFn     func(ctx context.Context, providerName string, id uuid.UUID) (*groups.Group, error)
	listFn    func(ctx context.Context, providerName, filterStr string, offset, limit int) (*groups.ListResult, error)
	replaceFn func(ctx context.Context, providerName string, id uuid.UUID, g groups.Group) (*groups.Group, error)
	patchFn   func(ctx context.Context, providerName string, id uuid.UUID, ops []groups.PatchOp) (*groups.Group, error)
	deleteFn  func(ctx context.Context, providerName string, id uuid.UUID) error
}

func (m *mockGroupsService) Create(ctx context.Context, providerName string, g groups.Group) (*groups.Group, error) {
	if m.createFn == nil {
		return nil, errNotImplemented
	}
	return m.createFn(ctx, providerName, g)
}

func (m *mockGroupsService) GetByEntityID(ctx context.Context, providerName string, id uuid.UUID) (*groups.Group, error) {
	if m.getFn == nil {
		return nil, errNotImplemented
	}
	return m.getFn(ctx, providerName, id)
}

func (m *mockGroupsService) List(ctx context.Context, providerName, filterStr string, offset, limit int) (*groups.ListResult, error) {
	if m.listFn == nil {
		return nil, errNotImplemented
	}
	return m.listFn(ctx, providerName, filterStr, offset, limit)
}

func (m *mockGroupsService) Replace(ctx context.Context, providerName string, id uuid.UUID, g groups.Group) (*groups.Group, error) {
	if m.replaceFn == nil {
		return nil, errNotImplemented
	}
	return m.replaceFn(ctx, providerName, id, g)
}

func (m *mockGroupsService) Patch(ctx context.Context, providerName string, id uuid.UUID, ops []groups.PatchOp) (*groups.Group, error) {
	if m.patchFn == nil {
		return nil, errNotImplemented
	}
	return m.patchFn(ctx, providerName, id, ops)
}

func (m *mockGroupsService) Delete(ctx context.Context, providerName string, id uuid.UUID) error {
	if m.deleteFn == nil {
		return errNotImplemented
	}
	return m.deleteFn(ctx, providerName, id)
}

func newGroupsAuthRouter(t *testing.T, svc handler.GroupsService) (router chi.Router, token string) {
	t.Helper()
	registry := scimauth.NewRegistry()
	token = "test-token"
	if err := registry.Register("keycloak", token); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}
	router = chi.NewRouter()
	handler.Mount(router, registry, handler.Handlers{Groups: handler.NewGroupsHandler(svc)})
	return router, token
}

func sampleGroup(id uuid.UUID) *groups.Group {
	t := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	stored := groups.StoredGroup{
		EntityID:        id,
		Properties:      map[string]any{groups.PropertyKeyDisplayName: "Engineering"},
		ExternalSubject: "eng-1",
		CreatedAt:       t,
		UpdatedAt:       t,
	}
	out := groups.FromStored(stored)
	return &out
}

func TestGroups_Create_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	memberID := uuid.New()
	svc := &mockGroupsService{
		createFn: func(_ context.Context, providerName string, g groups.Group) (*groups.Group, error) {
			if providerName != "keycloak" {
				t.Errorf("providerName = %q, want keycloak", providerName)
			}
			if g.DisplayName != "Engineering" {
				t.Errorf("DisplayName = %q, want Engineering", g.DisplayName)
			}
			if len(g.Members) != 1 || g.Members[0].Value != memberID.String() {
				t.Errorf("Members = %+v, want one member with value %s", g.Members, memberID)
			}
			return sampleGroup(id), nil
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"displayName":"Engineering","externalId":"eng-1","members":[{"value":"` + memberID.String() + `"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Groups", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/scim+json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); !strings.Contains(got, id.String()) {
		t.Errorf("Location header = %q, want to contain %q", got, id.String())
	}
	if got := rr.Header().Get("ETag"); !strings.HasPrefix(got, `W/"`) {
		t.Errorf("ETag = %q, want weak prefix", got)
	}
}

func TestGroups_Create_MissingDisplayName(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		createFn: func(_ context.Context, _ string, _ groups.Group) (*groups.Group, error) {
			return nil, groups.ErrDisplayNameRequired
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"externalId":"eng-1"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Groups", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestGroups_Create_InvalidMember(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		createFn: func(_ context.Context, _ string, _ groups.Group) (*groups.Group, error) {
			return nil, groups.ErrInvalidMember
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"displayName":"Eng","externalId":"eng-1","members":[{"value":"not-a-uuid"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Groups", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestGroups_Create_MalformedJSON(t *testing.T) {
	t.Parallel()

	r, token := newGroupsAuthRouter(t, &mockGroupsService{})

	body := strings.NewReader(`{not-json`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scim/v2/Groups", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestGroups_GetByID_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockGroupsService{
		getFn: func(_ context.Context, _ string, gotID uuid.UUID) (*groups.Group, error) {
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			return sampleGroup(id), nil
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Groups/"+id.String(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("ETag"); !strings.HasPrefix(got, `W/"`) {
		t.Errorf("ETag = %q, want weak prefix", got)
	}
	var decoded groups.Group
	if err := json.NewDecoder(rr.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.DisplayName != "Engineering" {
		t.Errorf("displayName = %q, want Engineering", decoded.DisplayName)
	}
}

func TestGroups_GetByID_BadUUID(t *testing.T) {
	t.Parallel()

	r, token := newGroupsAuthRouter(t, &mockGroupsService{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Groups/not-a-uuid", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestGroups_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		getFn: func(_ context.Context, _ string, _ uuid.UUID) (*groups.Group, error) {
			return nil, groups.ErrNotFound
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Groups/"+uuid.NewString(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestGroups_Delete_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	called := false
	svc := &mockGroupsService{
		deleteFn: func(_ context.Context, providerName string, gotID uuid.UUID) error {
			called = true
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			if providerName != "keycloak" {
				t.Errorf("provider = %q, want keycloak", providerName)
			}
			return nil
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/scim/v2/Groups/"+id.String(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Errorf("Delete was not invoked on the service")
	}
	if rr.Body.Len() != 0 {
		body, _ := io.ReadAll(rr.Body)
		t.Errorf("body = %q, want empty for 204", string(body))
	}
}

func TestGroups_Delete_Builtin(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		deleteFn: func(_ context.Context, _ string, _ uuid.UUID) error {
			return groups.ErrBuiltinGroup
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/scim/v2/Groups/"+uuid.NewString(), http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestGroups_List_Success(t *testing.T) {
	t.Parallel()

	gid := uuid.New()
	svc := &mockGroupsService{
		listFn: func(_ context.Context, providerName, filterStr string, offset, limit int) (*groups.ListResult, error) {
			if providerName != "keycloak" {
				t.Errorf("provider = %q, want keycloak", providerName)
			}
			if filterStr != `displayName eq "Engineering"` {
				t.Errorf("filter = %q, want displayName eq \"Engineering\"", filterStr)
			}
			if offset != 0 || limit != 50 {
				t.Errorf("paging = (offset=%d, limit=%d), want (0, 50)", offset, limit)
			}
			return &groups.ListResult{Groups: []*groups.Group{sampleGroup(gid)}, Total: 1}, nil
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		`/scim/v2/Groups?filter=displayName+eq+%22Engineering%22`, http.NoBody)
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

func TestGroups_List_PaginationClamps(t *testing.T) {
	t.Parallel()

	var gotOffset, gotLimit int
	svc := &mockGroupsService{
		listFn: func(_ context.Context, _, _ string, offset, limit int) (*groups.ListResult, error) {
			gotOffset = offset
			gotLimit = limit
			return &groups.ListResult{Groups: nil, Total: 0}, nil
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	// startIndex=0 (invalid, must clamp to 1 → offset 0); count=999 (clamps to 200).
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/scim/v2/Groups?startIndex=0&count=999", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotOffset != 0 || gotLimit != 200 {
		t.Errorf("paging = (offset=%d, limit=%d), want (0, 200)", gotOffset, gotLimit)
	}
}

func TestGroups_List_InvalidFilter(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		listFn: func(_ context.Context, _, _ string, _, _ int) (*groups.ListResult, error) {
			return nil, groups.ErrInvalidFilter
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/scim/v2/Groups?filter=bogus", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestGroups_Replace_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	memberID := uuid.New()
	svc := &mockGroupsService{
		replaceFn: func(_ context.Context, _ string, gotID uuid.UUID, g groups.Group) (*groups.Group, error) {
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			if g.DisplayName != "Engineering+" {
				t.Errorf("displayName = %q", g.DisplayName)
			}
			if len(g.Members) != 1 || g.Members[0].Value != memberID.String() {
				t.Errorf("members = %+v", g.Members)
			}
			return sampleGroup(id), nil
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"displayName":"Engineering+","members":[{"value":"` + memberID.String() + `"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/scim/v2/Groups/"+id.String(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/scim+json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("ETag"); !strings.HasPrefix(got, `W/"`) {
		t.Errorf("ETag = %q, want weak prefix", got)
	}
}

func TestGroups_Replace_BadUUID(t *testing.T) {
	t.Parallel()

	r, token := newGroupsAuthRouter(t, &mockGroupsService{})

	body := strings.NewReader(`{"displayName":"x"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/scim/v2/Groups/not-a-uuid", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestGroups_Replace_NotFound(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		replaceFn: func(_ context.Context, _ string, _ uuid.UUID, _ groups.Group) (*groups.Group, error) {
			return nil, groups.ErrNotFound
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"displayName":"x"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/scim/v2/Groups/"+uuid.NewString(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestGroups_Replace_Builtin(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		replaceFn: func(_ context.Context, _ string, _ uuid.UUID, _ groups.Group) (*groups.Group, error) {
			return nil, groups.ErrBuiltinGroup
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"displayName":"hijack"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/scim/v2/Groups/"+uuid.NewString(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestGroups_Patch_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	var gotOps []groups.PatchOp
	svc := &mockGroupsService{
		patchFn: func(_ context.Context, providerName string, gotID uuid.UUID, ops []groups.PatchOp) (*groups.Group, error) {
			if providerName != "keycloak" {
				t.Errorf("provider = %q, want keycloak", providerName)
			}
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			gotOps = ops
			return sampleGroup(id), nil
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"displayName","value":"Engineering Renamed"},{"op":"add","path":"members","value":[{"value":"33333333-3333-3333-3333-333333333333"}]}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Groups/"+id.String(), body)
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

func TestGroups_Patch_NoTarget(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		patchFn: func(context.Context, string, uuid.UUID, []groups.PatchOp) (*groups.Group, error) {
			return nil, groups.ErrPatchNoTarget
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"Operations":[{"op":"remove","path":"displayName"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Groups/"+uuid.NewString(), body)
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

func TestGroups_Patch_InvalidValue(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		patchFn: func(context.Context, string, uuid.UUID, []groups.PatchOp) (*groups.Group, error) {
			return nil, groups.ErrPatchInvalidValue
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"Operations":[{"op":"add","path":"members","value":[{"value":"not-a-uuid"}]}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Groups/"+uuid.NewString(), body)
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

func TestGroups_Patch_Builtin(t *testing.T) {
	t.Parallel()

	svc := &mockGroupsService{
		patchFn: func(context.Context, string, uuid.UUID, []groups.PatchOp) (*groups.Group, error) {
			return nil, groups.ErrBuiltinGroup
		},
	}
	r, token := newGroupsAuthRouter(t, svc)

	body := strings.NewReader(`{"Operations":[{"op":"replace","path":"displayName","value":"hijack"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Groups/"+uuid.NewString(), body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestGroups_Patch_BadUUID(t *testing.T) {
	t.Parallel()

	r, token := newGroupsAuthRouter(t, &mockGroupsService{})

	body := strings.NewReader(`{"Operations":[]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/scim/v2/Groups/not-a-uuid", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestGroups_Auth_MissingToken(t *testing.T) {
	t.Parallel()

	r, _ := newGroupsAuthRouter(t, &mockGroupsService{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Groups/"+uuid.NewString(), http.NoBody)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

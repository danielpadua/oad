package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/danielpadua/oad/internal/scim/auth"
	"github.com/danielpadua/oad/internal/scim/handler"
)

const scimContentType = "application/scim+json"

func newTestRouter() chi.Router {
	r := chi.NewRouter()
	handler.Mount(r, auth.NewRegistry(), handler.Handlers{})
	return r
}

// fetchServiceProviderConfig issues GET /scim/v2/ServiceProviderConfig
// against a fresh router and returns the response recorder plus the
// decoded body. Fails the test if the status is not 200 or the body is
// not valid JSON.
func fetchServiceProviderConfig(t *testing.T) (rr *httptest.ResponseRecorder, body map[string]any) {
	t.Helper()
	r := newTestRouter()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/ServiceProviderConfig", http.NoBody)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return rr, body
}

func TestServiceProviderConfig_ContentTypeAndSchemas(t *testing.T) {
	t.Parallel()
	rr, body := fetchServiceProviderConfig(t)

	if got := rr.Header().Get("Content-Type"); got != scimContentType {
		t.Errorf("Content-Type = %q, want %q", got, scimContentType)
	}
	schemas, _ := body["schemas"].([]any)
	if len(schemas) != 1 || schemas[0] != "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig" {
		t.Errorf("schemas = %v, want SCIM ServiceProviderConfig URN", schemas)
	}
}

func TestServiceProviderConfig_Capabilities(t *testing.T) {
	t.Parallel()
	_, body := fetchServiceProviderConfig(t)

	cases := []struct {
		field         string
		wantSupported bool
	}{
		{"patch", true},
		{"bulk", false},
		{"changePassword", false},
		{"sort", false},
		{"etag", true},
	}
	for _, tc := range cases {
		flag, _ := body[tc.field].(map[string]any)
		if flag["supported"] != tc.wantSupported {
			t.Errorf("%s.supported = %v, want %v", tc.field, flag["supported"], tc.wantSupported)
		}
	}
}

func TestServiceProviderConfig_Filter(t *testing.T) {
	t.Parallel()
	_, body := fetchServiceProviderConfig(t)

	filter, _ := body["filter"].(map[string]any)
	if filter["supported"] != true {
		t.Errorf("filter.supported = %v, want true", filter["supported"])
	}
	if filter["maxResults"].(float64) != 200 {
		t.Errorf("filter.maxResults = %v, want 200", filter["maxResults"])
	}
}

func TestServiceProviderConfig_AuthSchemes(t *testing.T) {
	t.Parallel()
	_, body := fetchServiceProviderConfig(t)

	authSchemes, _ := body["authenticationSchemes"].([]any)
	if len(authSchemes) != 1 {
		t.Fatalf("authenticationSchemes len = %d, want 1", len(authSchemes))
	}
	scheme, _ := authSchemes[0].(map[string]any)
	if scheme["type"] != "oauthbearertoken" || scheme["primary"] != true {
		t.Errorf("auth scheme = %v, want oauthbearertoken primary", scheme)
	}
}

func TestServiceProviderConfig_Meta(t *testing.T) {
	t.Parallel()
	_, body := fetchServiceProviderConfig(t)

	meta, _ := body["meta"].(map[string]any)
	if meta["resourceType"] != "ServiceProviderConfig" {
		t.Errorf("meta.resourceType = %v, want ServiceProviderConfig", meta["resourceType"])
	}
	if meta["location"] != "/scim/v2/ServiceProviderConfig" {
		t.Errorf("meta.location = %v, want /scim/v2/ServiceProviderConfig", meta["location"])
	}
}

func TestDiscovery_SchemasContainsUser(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Schemas", http.NoBody)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["totalResults"].(float64) != 1 {
		t.Errorf("totalResults = %v, want 1", body["totalResults"])
	}
	resources, _ := body["Resources"].([]any)
	if len(resources) != 1 {
		t.Fatalf("Resources len = %d, want 1", len(resources))
	}
	user, _ := resources[0].(map[string]any)
	if user["id"] != "urn:ietf:params:scim:schemas:core:2.0:User" {
		t.Errorf("User schema id = %v, want SCIM User URN", user["id"])
	}
	if user["name"] != "User" {
		t.Errorf("User schema name = %v, want User", user["name"])
	}
}

func TestDiscovery_ResourceTypesContainsUser(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/ResourceTypes", http.NoBody)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["totalResults"].(float64) != 1 {
		t.Errorf("totalResults = %v, want 1", body["totalResults"])
	}
	resources, _ := body["Resources"].([]any)
	if len(resources) != 1 {
		t.Fatalf("Resources len = %d, want 1", len(resources))
	}
	rt, _ := resources[0].(map[string]any)
	if rt["id"] != "User" || rt["endpoint"] != "/Users" {
		t.Errorf("ResourceType = %v, want id=User endpoint=/Users", rt)
	}
}

func TestDiscovery_EndpointsAreUnauthenticated(t *testing.T) {
	t.Parallel()

	// Discovery endpoints must respond 200 even with no Authorization header.
	// The Mount call in newTestRouter passes an empty registry — which means
	// any authenticated route would reject every token. The discovery
	// endpoints succeed regardless, proving they sit outside the auth group.
	r := newTestRouter()

	for _, path := range []string{"/scim/v2/ServiceProviderConfig", "/scim/v2/Schemas", "/scim/v2/ResourceTypes"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, rr.Code)
		}
	}
}

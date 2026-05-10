package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/handler"
	"github.com/danielpadua/oad/internal/entity"
	"github.com/danielpadua/oad/internal/useradmin"
)

type mockUserAdminService struct {
	listFn func(ctx context.Context, params useradmin.ListParams) (*useradmin.ListResult, error)
}

func (m *mockUserAdminService) List(ctx context.Context, params useradmin.ListParams) (*useradmin.ListResult, error) {
	if m.listFn == nil {
		return nil, errors.New("mock: list not implemented")
	}
	return m.listFn(ctx, params)
}

func TestUsersHandler_List_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserAdminService{
		listFn: func(ctx context.Context, params useradmin.ListParams) (*useradmin.ListResult, error) {
			if params.Limit != 25 || params.Offset != 0 {
				t.Errorf("expected limit=25, offset=0, got %d, %d", params.Limit, params.Offset)
			}
			tNow := time.Now()
			u := &useradmin.User{
				Entity: &entity.Entity{
					ID:         id,
					Type:       "User",
					Properties: []byte(`{"userName": "alice"}`),
					CreatedAt:  tNow,
					UpdatedAt:  tNow,
				},
				ExternalIdentities: []useradmin.ExternalIdentity{
					{ProviderName: "keycloak", ExternalSubject: "alice-kc"},
				},
			}
			return &useradmin.ListResult{
				Items:  []*useradmin.User{u},
				Total:  1,
				Limit:  25,
				Offset: 0,
			}, nil
		},
	}

	h := handler.NewUsersHandler(svc)
	r := chi.NewRouter()
	r.Get("/users", h.List)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var res useradmin.ListResult
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Total != 1 {
		t.Errorf("expected 1 user, got %d", res.Total)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item in list, got %d", len(res.Items))
	}
	if res.Items[0].ID != id {
		t.Errorf("expected id %v, got %v", id, res.Items[0].ID)
	}
	if len(res.Items[0].ExternalIdentities) != 1 {
		t.Fatalf("expected 1 external identity, got %d", len(res.Items[0].ExternalIdentities))
	}
	if res.Items[0].ExternalIdentities[0].ProviderName != "keycloak" {
		t.Errorf("expected keycloak provider, got %s", res.Items[0].ExternalIdentities[0].ProviderName)
	}
}

func TestUsersHandler_List_Error(t *testing.T) {
	t.Parallel()

	svc := &mockUserAdminService{
		listFn: func(ctx context.Context, params useradmin.ListParams) (*useradmin.ListResult, error) {
			return nil, errors.New("db error")
		},
	}

	h := handler.NewUsersHandler(svc)
	r := chi.NewRouter()
	r.Get("/users", h.List)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

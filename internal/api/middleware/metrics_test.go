package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/danielpadua/oad/internal/api/middleware"
)

func TestMetricsMiddleware_IncrementsCounterAndHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := middleware.NewMetricsMiddleware(reg)

	r := chi.NewRouter()
	r.Use(m.Handler)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := make(map[string]bool)
	for _, mf := range metrics {
		found[mf.GetName()] = true
	}

	for _, name := range []string{"http_requests_total", "http_request_duration_seconds", "http_requests_in_flight"} {
		if !found[name] {
			t.Errorf("expected metric %q to be registered", name)
		}
	}
}

func TestMetricsMiddleware_RecordsStatusCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := middleware.NewMetricsMiddleware(reg)

	r := chi.NewRouter()
	r.Use(m.Handler)
	r.Get("/not-found", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/not-found", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	for _, mf := range metrics {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "status" && lp.GetValue() == "404" {
					return // found the expected label
				}
			}
		}
	}
	t.Error("expected http_requests_total with status=404 label")
}

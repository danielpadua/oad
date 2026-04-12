package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsMiddleware collects HTTP request metrics for Prometheus:
// total request count, latency histograms, and in-flight gauge.
type MetricsMiddleware struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	inFlight        prometheus.Gauge
}

// NewMetricsMiddleware registers Prometheus metrics on the given registerer
// and returns middleware ready to be mounted on the router.
func NewMetricsMiddleware(reg prometheus.Registerer) *MetricsMiddleware {
	factory := promauto.With(reg)

	return &MetricsMiddleware{
		requestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status.",
		}, []string{"method", "path", "status"}),

		requestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		inFlight: factory.NewGauge(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served.",
		}),
	}
}

// Handler returns the Chi-compatible middleware function.
func (m *MetricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Inc()
		defer m.inFlight.Dec()

		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		// Use the Chi route pattern (e.g., "/api/v1/entities/{id}") instead
		// of the raw URL path to prevent high-cardinality label explosion.
		path := r.URL.Path
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if pattern := rctx.RoutePattern(); pattern != "" {
				path = pattern
			}
		}

		status := strconv.Itoa(wrapped.statusCode)
		m.requestsTotal.WithLabelValues(r.Method, path, status).Inc()
		m.requestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

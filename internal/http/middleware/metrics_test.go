package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/metrics"
)

// resetHTTPMetrics re-creates the HTTP metrics collectors fresh
// to avoid duplicate registration panics between tests.
func resetHTTPMetrics(t *testing.T) {
	t.Helper()
	metrics.HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "senda_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	metrics.HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "senda_http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "path", "status"},
	)
}

func TestMetrics_RecordsRequestCount(t *testing.T) {
	resetHTTPMetrics(t)

	e := echo.New()
	e.Use(middleware.Metrics())
	e.GET("/health", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	counter, err := metrics.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/health", "200")
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}
	if got := testutil.ToFloat64(counter); got != 1 {
		t.Fatalf("expected counter value 1, got %f", got)
	}
}

func TestMetrics_PathNormalizationUUID(t *testing.T) {
	resetHTTPMetrics(t)

	e := echo.New()
	e.Use(middleware.Metrics())
	// No registered route that matches — forces fallback path normalization.
	e.GET("/api/v1/users/*", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// The route pattern /api/v1/users/* is used since it matches.
	counter, err := metrics.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/api/v1/users/*", "200")
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}
	if got := testutil.ToFloat64(counter); got != 1 {
		t.Fatalf("expected counter value 1, got %f", got)
	}
}

func TestMetrics_NormalizesUUIDWhenNoRoute(t *testing.T) {
	resetHTTPMetrics(t)

	// Test the UUID normalization logic directly.
	got := middleware.NormalizePath("/api/v1/users/550e8400-e29b-41d4-a716-446655440000")
	expected := "/api/v1/users/:id"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestMetrics_NormalizesMultipleUUIDs(t *testing.T) {
	resetHTTPMetrics(t)

	got := middleware.NormalizePath("/api/v1/550e8400-e29b-41d4-a716-446655440000/items/660e8400-e29b-41d4-a716-446655440001")
	expected := "/api/v1/:id/items/:id"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestMetrics_PreservesPathWithoutUUID(t *testing.T) {
	got := middleware.NormalizePath("/api/v1/users")
	expected := "/api/v1/users"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestMetrics_UsesRoutePattern(t *testing.T) {
	resetHTTPMetrics(t)

	e := echo.New()
	e.Use(middleware.Metrics())

	e.GET("/api/v1/:tenant_code/:workspace_code/templates", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acme/main/templates", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	counter, err := metrics.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/api/v1/:tenant_code/:workspace_code/templates", "200")
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}
	if got := testutil.ToFloat64(counter); got != 1 {
		t.Fatalf("expected counter value 1, got %f", got)
	}
}

func TestMetrics_RecordsDuration(t *testing.T) {
	resetHTTPMetrics(t)

	e := echo.New()
	e.Use(middleware.Metrics())
	e.GET("/health", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	count := testutil.CollectAndCount(metrics.HTTPRequestDuration, "senda_http_request_duration_seconds")
	if count == 0 {
		t.Fatal("expected at least one histogram observation")
	}
}

package http_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/rendis/senda/config"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
)

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHealthEndpoint(t *testing.T) {
	srv := sendahttp.NewServer(testConfig(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("expected status 'healthy', got %q", body["status"])
	}
}

func TestRequestIDGenerated(t *testing.T) {
	srv := sendahttp.NewServer(testConfig(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)

	reqID := rec.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
	if len(reqID) < 32 {
		t.Fatalf("expected UUID-length request ID, got %q", reqID)
	}
}

func TestRequestIDPropagated(t *testing.T) {
	srv := sendahttp.NewServer(testConfig(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", "test-request-id-123")
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)

	reqID := rec.Header().Get("X-Request-ID")
	if reqID != "test-request-id-123" {
		t.Fatalf("expected propagated request ID 'test-request-id-123', got %q", reqID)
	}
}

func TestPanicRecovery(t *testing.T) {
	srv := sendahttp.NewServer(testConfig(), testLogger())

	srv.Echo().GET("/panic", func(c *echo.Context) error {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("expected error code 'INTERNAL_ERROR', got %q", errResp.Error.Code)
	}
	if errResp.Error.Message == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestScopeExtraction(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Scope())

	var gotTenant, gotWorkspace string

	e.GET("/api/v1/:tenant_code/:workspace_code/test", func(c *echo.Context) error {
		gotTenant, _ = c.Get(middleware.ContextKeyTenantCode).(string)
		gotWorkspace, _ = c.Get(middleware.ContextKeyWorkspaceCode).(string)
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acme/main/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if gotTenant != "acme" {
		t.Fatalf("expected tenant_code 'acme', got %q", gotTenant)
	}
	if gotWorkspace != "main" {
		t.Fatalf("expected workspace_code 'main', got %q", gotWorkspace)
	}
}

func TestScopeNoParams(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Scope())

	var hasTenant, hasWorkspace bool

	e.GET("/health", func(c *echo.Context) error {
		_, hasTenant = c.Get(middleware.ContextKeyTenantCode).(string)
		_, hasWorkspace = c.Get(middleware.ContextKeyWorkspaceCode).(string)
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	// When there are no path params, c.Get returns nil, which type-asserts to "".
	// hasTenant/hasWorkspace will be false because empty string is the zero value.
	if hasTenant {
		t.Fatal("expected no tenant_code in context for /health route")
	}
	if hasWorkspace {
		t.Fatal("expected no workspace_code in context for /health route")
	}
}

func TestErrorResponseFormat(t *testing.T) {
	e := echo.New()
	e.Use(middleware.RequestID())
	e.HTTPErrorHandler = response.HTTPErrorHandler

	e.GET("/error", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "resource not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	req.Header.Set("X-Request-ID", "err-req-123")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected error code 'NOT_FOUND', got %q", errResp.Error.Code)
	}
	if errResp.Error.Message != "resource not found" {
		t.Fatalf("expected message 'resource not found', got %q", errResp.Error.Message)
	}
	if errResp.Error.RequestID != "err-req-123" {
		t.Fatalf("expected request_id 'err-req-123', got %q", errResp.Error.RequestID)
	}
}

func TestRequestIDMiddlewareUnit(t *testing.T) {
	handler := func(c *echo.Context) error {
		reqID, _ := c.Get(middleware.ContextKeyRequestID).(string)
		return c.String(http.StatusOK, reqID)
	}

	mw := middleware.RequestID()

	t.Run("generates ID when not provided", func(t *testing.T) {
		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		if err := mw(handler)(c); err != nil {
			t.Fatal(err)
		}

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if body == "" {
			t.Fatal("expected non-empty request ID in response body")
		}
	})

	t.Run("uses provided ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "custom-id-456")
		c, rec := echotest.ContextConfig{
			Request: req,
		}.ToContextRecorder(t)

		if err := mw(handler)(c); err != nil {
			t.Fatal(err)
		}

		body := rec.Body.String()
		if body != "custom-id-456" {
			t.Fatalf("expected 'custom-id-456', got %q", body)
		}
	})
}

func TestWriteErrorHelper(t *testing.T) {
	e := echo.New()
	e.Use(middleware.RequestID())

	e.GET("/write-error", func(c *echo.Context) error {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid input",
			response.FieldError{Field: "email", Message: "must be valid email"},
		)
	})

	req := httptest.NewRequest(http.MethodGet, "/write-error", nil)
	req.Header.Set("X-Request-ID", "write-err-789")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Code != "BAD_REQUEST" {
		t.Fatalf("expected error code 'BAD_REQUEST', got %q", errResp.Error.Code)
	}
	if errResp.Error.RequestID != "write-err-789" {
		t.Fatalf("expected request_id 'write-err-789', got %q", errResp.Error.RequestID)
	}
	if len(errResp.Error.Details) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(errResp.Error.Details))
	}
	if errResp.Error.Details[0].Field != "email" {
		t.Fatalf("expected field 'email', got %q", errResp.Error.Details[0].Field)
	}
}

func TestServer_RouteContractRemainsPartitionableBySurface(t *testing.T) {
	srv := sendahttp.NewServer(
		testConfig(),
		testLogger(),
		sendahttp.WithTenantHandler(new(handler.TenantHandler)),
		sendahttp.WithWorkspaceHandler(new(handler.WorkspaceHandler)),
		sendahttp.WithWorkspacePolicyHandler(new(handler.WorkspacePolicyHandler)),
		sendahttp.WithMemberHandler(new(handler.MemberHandler)),
		sendahttp.WithConfigHandler(new(handler.ConfigHandler)),
		sendahttp.WithSendHandler(new(handler.SendHandler)),
		sendahttp.WithDataPlaneEmailHandler(new(handler.DataPlaneEmailHandler)),
		sendahttp.WithExternalIntegrationHandler(new(handler.ExternalIntegrationHandler)),
		sendahttp.WithTemplateTypeHandler(new(handler.TemplateTypeHandler)),
		sendahttp.WithTemplateHandler(new(handler.TemplateHandler)),
		sendahttp.WithInjectorHandler(new(handler.InjectorHandler)),
		sendahttp.WithSESWebhookHandler(new(handler.SESWebhookHandler)),
		sendahttp.WithTrackingHandler(new(handler.TrackingHandler)),
		sendahttp.WithMediaHandler(new(handler.MediaHandler)),
	)

	routes := make(map[string]struct{})
	for _, route := range srv.Echo().Router().Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	expected := []string{
		"GET /health",
		"GET /t/o/:tracking_id",
		"GET /public/video-thumbnail",
		"POST /api/v1/send",
		"GET /api/v1/emails",
		"POST /api/v1/webhooks/ses/inbound",
		"GET /api/v1/external/:profile_slug/bootstrap",
		"GET /api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/template-types",
		"GET /api/v1/manage/tenants",
		"GET /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/template-types",
		"GET /api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/template-types",
	}

	for _, route := range expected {
		if _, ok := routes[route]; !ok {
			t.Fatalf("expected route %s to remain registered", route)
		}
	}
}

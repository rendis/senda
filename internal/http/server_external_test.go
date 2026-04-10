package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

type externalConfigStore struct {
	getFn func(context.Context) (*domain.GlobalConfig, error)
}

func (m *externalConfigStore) Get(ctx context.Context) (*domain.GlobalConfig, error) {
	if m.getFn != nil {
		return m.getFn(ctx)
	}
	return &domain.GlobalConfig{}, nil
}

func (m *externalConfigStore) Upsert(context.Context, *domain.GlobalConfig) error { return nil }

type externalAuthMethod struct {
	name        string
	description string
	result      *port.ExternalAuthResult
	err         error
}

func (m *externalAuthMethod) Name() string        { return m.name }
func (m *externalAuthMethod) Description() string { return m.description }
func (m *externalAuthMethod) Authenticate(context.Context, *port.ExternalIntegrationRequest) (*port.ExternalAuthResult, error) {
	if m.result == nil && m.err == nil {
		return &port.ExternalAuthResult{}, nil
	}
	return m.result, m.err
}

type externalResolver struct {
	name        string
	description string
	result      *port.ExternalWorkspaceResolution
	err         error
}

func (r *externalResolver) Name() string        { return r.name }
func (r *externalResolver) Description() string { return r.description }
func (r *externalResolver) ResolveWorkspace(context.Context, *port.ExternalIntegrationRequest, *port.ExternalAuthResult) (*port.ExternalWorkspaceResolution, error) {
	if r.result == nil && r.err == nil {
		return &port.ExternalWorkspaceResolution{WorkspaceCode: "main"}, nil
	}
	return r.result, r.err
}

func externalProfileConfig() *domain.GlobalConfig {
	return &domain.GlobalConfig{
		ExternalIntegrations: []domain.ExternalIntegrationProfile{
			{
				Slug:            "partner-portal",
				Name:            "Partner Portal",
				Description:     "External integration",
				Enabled:         true,
				AuthMethodName:  "signed-headers",
				ResolverName:    "tenant-workspace-resolver",
				AllowedOrigins:  []string{"https://app.example.com"},
				AllowedHeaders:  []string{"x-tenant-code", "x-signature"},
				RequiredHeaders: []string{"x-tenant-code"},
				Capabilities: domain.ExternalIntegrationCapabilities{
					ListTemplates:   true,
					ViewVersions:    true,
					EditVersions:    true,
					PublishVersions: true,
					TestSend:        true,
					BuilderAccess:   true,
					MetadataAccess:  true,
					LocaleAccess:    true,
				},
			},
		},
	}
}

func setupExternalRouteServer(cfg *domain.GlobalConfig, auth *externalAuthMethod, resolver *externalResolver) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())

	h := handler.NewExternalIntegrationHandler(&externalConfigStore{
		getFn: func(context.Context) (*domain.GlobalConfig, error) {
			return cfg, nil
		},
	}, []port.ExternalAuthMethod{auth}, []port.ExternalWorkspaceResolver{resolver})

	e.Use(middleware.ExternalIntegrationCORS(h))

	group := e.Group("/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code")
	group.Use(middleware.ExternalIntegration(h))

	group.GET("/template-types", func(c *echo.Context) error {
		state := map[string]any{
			"workspace_code": c.Param("workspace_code"),
			"read_only":      c.Get(middleware.ContextKeyExternalIntegrationReadOnly),
		}
		return c.JSON(http.StatusOK, state)
	}, middleware.RequireExternalCapability(middleware.ExternalActionMetadataAccess))

	group.PUT("/templates/:template_id/versions/:version_id", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"workspace_code": c.Param("workspace_code"),
			"read_only":      c.Get(middleware.ContextKeyExternalIntegrationReadOnly),
		})
	}, middleware.RequireExternalCapability(middleware.ExternalActionEditVersions), middleware.RequireExternalMutation())

	group.POST("/templates/:template_id/test-send", func(c *echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, middleware.RequireExternalCapability(middleware.ExternalActionTestSend))

	group.POST("/templates/:template_id/preview-mjml", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"workspace_code": c.Param("workspace_code"),
			"read_only":      c.Get(middleware.ContextKeyExternalIntegrationReadOnly),
		})
	}, middleware.RequireExternalCapability(middleware.ExternalActionViewVersions))

	group.GET("/policies", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"workspace_code": c.Param("workspace_code"),
			"read_only":      c.Get(middleware.ContextKeyExternalIntegrationReadOnly),
		})
	}, middleware.RequireExternalCapability(middleware.ExternalActionBuilderAccess))

	return e
}

func TestServer_ExternalIntegrationSurface_BootstrapRemainsOutsideOIDC(t *testing.T) {
	srv := sendahttp.NewServer(testConfig(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/bootstrap", nil)
	rec := httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("external integration route must not be behind OIDC middleware, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	rec = httptest.NewRecorder()
	srv.Echo().ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected global frame protection to remain DENY, got %q", got)
	}
}

func TestExternalIntegrationMiddleware_SuccessAndPathRewrite(t *testing.T) {
	cfg := externalProfileConfig()
	auth := &externalAuthMethod{
		name:        "signed-headers",
		description: "Signed headers auth",
		result: &port.ExternalAuthResult{
			Permissions: port.ExternalPermissions{
				ListTemplates:   true,
				ViewVersions:    true,
				EditVersions:    true,
				PublishVersions: true,
				TestSend:        true,
				BuilderAccess:   true,
				MetadataAccess:  true,
				LocaleAccess:    true,
			},
		},
	}
	resolver := &externalResolver{
		name:        "tenant-workspace-resolver",
		description: "Tenant workspace resolver",
		result:      &port.ExternalWorkspaceResolution{WorkspaceCode: "main"},
	}
	e := setupExternalRouteServer(cfg, auth, resolver)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("X-Signature", "sig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["workspace_code"] != "main" {
		t.Fatalf("expected workspace_code to remain main, got %#v", body["workspace_code"])
	}
}

func TestExternalIntegrationMiddleware_AllowsPoliciesReadForBuilder(t *testing.T) {
	cfg := externalProfileConfig()
	auth := &externalAuthMethod{
		name:        "signed-headers",
		description: "Signed headers auth",
		result: &port.ExternalAuthResult{
			Permissions: port.ExternalPermissions{
				BuilderAccess: true,
			},
		},
	}
	resolver := &externalResolver{
		name:        "tenant-workspace-resolver",
		description: "Tenant workspace resolver",
		result:      &port.ExternalWorkspaceResolution{WorkspaceCode: "main"},
	}
	e := setupExternalRouteServer(cfg, auth, resolver)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/policies", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("X-Signature", "sig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExternalIntegrationMiddleware_DisabledProfile(t *testing.T) {
	cfg := externalProfileConfig()
	cfg.ExternalIntegrations[0].Enabled = false

	e := setupExternalRouteServer(cfg, &externalAuthMethod{name: "signed-headers"}, &externalResolver{name: "tenant-workspace-resolver"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("X-Signature", "sig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled profile, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExternalIntegrationMiddleware_MissingRequiredHeader(t *testing.T) {
	e := setupExternalRouteServer(externalProfileConfig(), &externalAuthMethod{name: "signed-headers"}, &externalResolver{name: "tenant-workspace-resolver"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	req.Header.Set("X-Signature", "sig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing required header, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExternalIntegrationMiddleware_AuthDenyAndError(t *testing.T) {
	cfg := externalProfileConfig()

	denyAuth := &externalAuthMethod{
		name:        "signed-headers",
		description: "Signed headers auth",
		err:         middleware.ExternalAuthDenied(),
	}
	e := setupExternalRouteServer(cfg, denyAuth, &externalResolver{name: "tenant-workspace-resolver"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("X-Signature", "sig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for auth deny, got %d: %s", rec.Code, rec.Body.String())
	}

	errAuth := &externalAuthMethod{
		name:        "signed-headers",
		description: "Signed headers auth",
		err:         errors.New("boom"),
	}
	e = setupExternalRouteServer(cfg, errAuth, &externalResolver{name: "tenant-workspace-resolver"})
	req = httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("X-Signature", "sig")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for auth error, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExternalIntegrationMiddleware_ResolverMismatch(t *testing.T) {
	cfg := externalProfileConfig()
	auth := &externalAuthMethod{name: "signed-headers", result: &port.ExternalAuthResult{}}
	resolver := &externalResolver{
		name:   "tenant-workspace-resolver",
		result: &port.ExternalWorkspaceResolution{WorkspaceCode: "main"},
	}
	e := setupExternalRouteServer(cfg, auth, resolver)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/other/template-types", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("X-Signature", "sig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for resolver mismatch, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExternalIntegrationMiddleware_FallbackReadOnlyAllowsReadBlocksMutation(t *testing.T) {
	cfg := externalProfileConfig()
	auth := &externalAuthMethod{
		name: "signed-headers",
		result: &port.ExternalAuthResult{
			Permissions: port.ExternalPermissions{
				ListTemplates:   true,
				ViewVersions:    true,
				EditVersions:    true,
				PublishVersions: true,
				TestSend:        true,
				BuilderAccess:   true,
				MetadataAccess:  true,
				LocaleAccess:    true,
			},
		},
	}
	resolver := &externalResolver{
		name:   "tenant-workspace-resolver",
		result: &port.ExternalWorkspaceResolution{WorkspaceCode: "_system", ReadOnly: true},
	}
	e := setupExternalRouteServer(cfg, auth, resolver)

	readReq := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	readReq.Header.Set("X-Tenant-Code", "acme")
	readReq.Header.Set("X-Signature", "sig")
	readRec := httptest.NewRecorder()
	e.ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for read in read-only mode, got %d: %s", readRec.Code, readRec.Body.String())
	}

	var readBody map[string]any
	if err := json.NewDecoder(readRec.Body).Decode(&readBody); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if readBody["workspace_code"] != "_system" {
		t.Fatalf("expected read-only fallback to rewrite workspace_code to _system, got %#v", readBody["workspace_code"])
	}

	mutReq := httptest.NewRequest(http.MethodPut, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/templates/t1/versions/v1", nil)
	mutReq.Header.Set("X-Tenant-Code", "acme")
	mutReq.Header.Set("X-Signature", "sig")
	mutRec := httptest.NewRecorder()
	e.ServeHTTP(mutRec, mutReq)
	if mutRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for mutation in read-only mode, got %d: %s", mutRec.Code, mutRec.Body.String())
	}
}

func TestExternalIntegrationCORS_PreflightHonorsProfileOriginsAndHeaders(t *testing.T) {
	e := setupExternalRouteServer(
		externalProfileConfig(),
		&externalAuthMethod{name: "signed-headers"},
		&externalResolver{name: "tenant-workspace-resolver"},
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	req.Header.Set(echo.HeaderOrigin, "https://app.example.com")
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodGet)
	req.Header.Set(echo.HeaderAccessControlRequestHeaders, "x-tenant-code, x-senda-external-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for allowed preflight, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get(echo.HeaderAccessControlAllowOrigin); got != "https://app.example.com" {
		t.Fatalf("expected profile origin to be echoed, got %q", got)
	}
	allowedHeaders := rec.Header().Get(echo.HeaderAccessControlAllowHeaders)
	if !strings.Contains(allowedHeaders, "X-Tenant-Code") {
		t.Fatalf("expected allow headers to contain X-Tenant-Code, got %q", allowedHeaders)
	}
	if !strings.Contains(allowedHeaders, "X-Senda-External-Token") {
		t.Fatalf("expected allow headers to contain X-Senda-External-Token, got %q", allowedHeaders)
	}
}

func TestExternalIntegrationCORS_DeniesUnexpectedOrigin(t *testing.T) {
	e := setupExternalRouteServer(
		externalProfileConfig(),
		&externalAuthMethod{name: "signed-headers"},
		&externalResolver{name: "tenant-workspace-resolver"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/main/template-types", nil)
	req.Header.Set(echo.HeaderOrigin, "https://evil.example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unexpected origin, got %d: %s", rec.Code, rec.Body.String())
	}
}

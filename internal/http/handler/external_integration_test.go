package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

type testExternalAuthMethod struct {
	name        string
	description string
}

func (t *testExternalAuthMethod) Name() string        { return t.name }
func (t *testExternalAuthMethod) Description() string { return t.description }
func (t *testExternalAuthMethod) Authenticate(context.Context, *port.ExternalIntegrationRequest) (*port.ExternalAuthResult, error) {
	return &port.ExternalAuthResult{}, nil
}

type testExternalResolver struct {
	name        string
	description string
}

func (t *testExternalResolver) Name() string        { return t.name }
func (t *testExternalResolver) Description() string { return t.description }
func (t *testExternalResolver) ResolveWorkspace(context.Context, *port.ExternalIntegrationRequest, *port.ExternalAuthResult) (*port.ExternalWorkspaceResolution, error) {
	return &port.ExternalWorkspaceResolution{WorkspaceCode: "_system", ReadOnly: true}, nil
}

func setupExternalIntegrationTest(cs *mockGlobalConfigStore, auths []port.ExternalAuthMethod, resolvers []port.ExternalWorkspaceResolver) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	h := handler.NewExternalIntegrationHandler(cs, auths, resolvers)
	e.GET("/api/v1/external/:profile_slug/bootstrap", h.Bootstrap)
	return e
}

func TestExternalIntegrationHandler_Bootstrap_MinimizesResponseMetadata(t *testing.T) {
	cfg := &domain.GlobalConfig{
		ExternalIntegrations: []domain.ExternalIntegrationProfile{
			{
				Slug:            "partner-portal",
				Name:            "Partner Portal",
				Description:     "External integration",
				Enabled:         true,
				AuthMethodName:  "signed-headers",
				ResolverName:    "tenant-workspace-resolver",
				AllowedOrigins:  []string{"https://app.example.com"},
				AllowedHeaders:  []string{"x-tenant-code"},
				RequiredHeaders: []string{"x-tenant-code"},
			},
		},
	}

	e := setupExternalIntegrationTest(&mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) { return cfg, nil },
	}, []port.ExternalAuthMethod{&testExternalAuthMethod{name: "signed-headers"}}, []port.ExternalWorkspaceResolver{&testExternalResolver{name: "tenant-workspace-resolver"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/bootstrap", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := resp["frame_ancestors"]; !ok {
		t.Fatalf("expected frame_ancestors in bootstrap response, got %+v", resp)
	}
	if _, ok := resp["auth_methods"]; ok {
		t.Fatalf("bootstrap must not expose auth methods metadata, got %+v", resp)
	}
	if _, ok := resp["workspace_resolvers"]; ok {
		t.Fatalf("bootstrap must not expose resolver metadata, got %+v", resp)
	}
	if _, ok := resp["profile"]; ok {
		t.Fatalf("bootstrap must not expose profile metadata, got %+v", resp)
	}
}

func TestExternalIntegrationHandler_Bootstrap_SetsFrameAncestors(t *testing.T) {
	cfg := &domain.GlobalConfig{
		ExternalIntegrations: []domain.ExternalIntegrationProfile{
			{
				Slug:           "partner-portal",
				Name:           "Partner Portal",
				Description:    "External integration",
				Enabled:        true,
				AuthMethodName: "signed-headers",
				ResolverName:   "tenant-workspace-resolver",
				AllowedOrigins: []string{"https://app.example.com", "https://admin.example.com"},
			},
		},
	}

	auth := &testExternalAuthMethod{name: "signed-headers", description: "Signed headers auth"}
	resolver := &testExternalResolver{name: "tenant-workspace-resolver", description: "Tenant workspace resolver"}

	e := setupExternalIntegrationTest(&mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) { return cfg, nil },
	}, []port.ExternalAuthMethod{auth}, []port.ExternalWorkspaceResolver{resolver})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/bootstrap", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("expected X-Frame-Options to be removed for bootstrap, got %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("expected bootstrap to set a frame-ancestors CSP")
	}

	var resp response.ExternalIntegrationBootstrapResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.FrameAncestors) != 3 {
		t.Fatalf("expected only frame ancestors in bootstrap response, got %+v", resp)
	}
}

func TestExternalIntegrationHandler_DisabledProfileRejected(t *testing.T) {
	cfg := &domain.GlobalConfig{
		ExternalIntegrations: []domain.ExternalIntegrationProfile{
			{
				Slug:           "partner-portal",
				Name:           "Partner Portal",
				Description:    "External integration",
				Enabled:        false,
				AuthMethodName: "signed-headers",
				ResolverName:   "tenant-workspace-resolver",
			},
		},
	}

	e := setupExternalIntegrationTest(&mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) { return cfg, nil },
	}, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/bootstrap", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled profile, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExternalIntegrationHandler_SessionReturnsEffectivePermissions(t *testing.T) {
	cfg := &domain.GlobalConfig{
		ExternalIntegrations: []domain.ExternalIntegrationProfile{
			{
				Slug:           "partner-portal",
				Name:           "Partner Portal",
				Description:    "External integration",
				Enabled:        true,
				AuthMethodName: "signed-headers",
				ResolverName:   "tenant-workspace-resolver",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/marketing/session", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextKeyExternalIntegrationReadOnly, false)
	c.Set(middleware.ContextKeyExternalIntegrationEffectiveWorkspaceCode, "marketing")
	c.Set(middleware.ContextKeyExternalIntegrationPermissions, port.ExternalPermissions{
		ListTemplates:   true,
		ViewVersions:    true,
		EditVersions:    false,
		PublishVersions: false,
		TestSend:        false,
		BuilderAccess:   true,
		MetadataAccess:  true,
		LocaleAccess:    true,
	})

	if err := handler.NewExternalIntegrationHandler(&mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) { return cfg, nil },
	}, nil, nil).Session(c); err != nil {
		t.Fatalf("session handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.ExternalIntegrationSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.EffectiveWorkspaceCode != "marketing" {
		t.Fatalf("expected effective workspace marketing, got %+v", resp)
	}
	if resp.Permissions.EditVersions {
		t.Fatalf("expected edit_versions=false, got %+v", resp.Permissions)
	}
	if !resp.Permissions.BuilderAccess {
		t.Fatalf("expected builder_access=true, got %+v", resp.Permissions)
	}
}

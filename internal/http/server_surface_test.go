package http_test

import (
	"net/http"
	"testing"

	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
)

func TestServer_ManagementSurface_RegistersWhenConfigHandlerPresent(t *testing.T) {
	srv := sendahttp.NewServer(
		testConfig(),
		testLogger(),
		sendahttp.WithConfigHandler(handler.NewConfigHandler(nil, handler.OIDCInfo{})),
	)

	if !routeExists(srv, http.MethodGet, "/api/v1/manage/config") {
		t.Fatalf("expected management config route to be registered when config handler is present")
	}
}

func TestServer_DataPlaneSurface_DoesNotLeakManagementRoutes(t *testing.T) {
	srv := sendahttp.NewServer(
		testConfig(),
		testLogger(),
		sendahttp.WithSendHandler(handler.NewSendHandler(nil, 10)),
		sendahttp.WithDataPlaneEmailHandler(handler.NewDataPlaneEmailHandler(nil)),
	)

	if !routeExists(srv, http.MethodPost, "/api/v1/send") {
		t.Fatalf("expected data-plane send route to be registered")
	}
	if routeExists(srv, http.MethodGet, "/api/v1/manage/config") {
		t.Fatalf("did not expect management route to be registered for data-plane-only server")
	}
}

func TestServer_ManagementSurface_RegistersEnvironmentWorkspaceRoutesWithWorkspaceHandlerOnly(t *testing.T) {
	srv := sendahttp.NewServer(
		testConfig(),
		testLogger(),
		sendahttp.WithWorkspaceHandler(handler.NewWorkspaceHandler(nil, nil, nil)),
	)

	if !routeExists(srv, http.MethodGet, "/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code") {
		t.Fatalf("expected environment-scoped management workspace route to be registered")
	}
	if routeExists(srv, http.MethodGet, "/api/v1/manage/tenants") {
		t.Fatalf("did not expect tenant management routes without a tenant handler")
	}
	if routeExists(srv, http.MethodPost, "/api/v1/send") {
		t.Fatalf("did not expect data-plane route to be registered for management-only server")
	}
}

func TestServer_DataPlaneSurface_RegistersOnboardingWithoutManagementLeak(t *testing.T) {
	srv := sendahttp.NewServer(
		testConfig(),
		testLogger(),
		sendahttp.WithOnboardingHandler(handler.NewOnboardingHandler(nil, nil)),
	)

	if !routeExists(srv, http.MethodGet, "/api/v1/onboarding/status") {
		t.Fatalf("expected onboarding status route to be registered")
	}
	if routeExists(srv, http.MethodGet, "/api/v1/manage/config") {
		t.Fatalf("did not expect management route to be registered for onboarding-only server")
	}
}

func TestServer_ExternalIntegrationSurface_RegistersSessionWithoutBuilderRoutes(t *testing.T) {
	srv := sendahttp.NewServer(
		testConfig(),
		testLogger(),
		sendahttp.WithExternalIntegrationHandler(handler.NewExternalIntegrationHandler(nil, nil, nil)),
	)

	if !routeExists(srv, http.MethodGet, "/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/session") {
		t.Fatalf("expected external integration session route to be registered")
	}
	if routeExists(srv, http.MethodGet, "/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/template-types") {
		t.Fatalf("did not expect builder routes to be registered without builder handlers")
	}
}

func routeExists(srv *sendahttp.Server, method, path string) bool {
	for _, route := range srv.Echo().Router().Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

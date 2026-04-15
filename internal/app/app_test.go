package app

import (
	"log/slog"
	"os"
	"testing"

	"github.com/rendis/senda/config"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
)

func TestNewServerOptions_RegistersExpectedSurfaces(t *testing.T) {
	opts := newServerOptions(serverSharedDeps{}, serverHandlerBundle{
		tenantHandler:              new(handler.TenantHandler),
		workspaceHandler:           new(handler.WorkspaceHandler),
		workspacePolicyHandler:     new(handler.WorkspacePolicyHandler),
		memberHandler:              new(handler.MemberHandler),
		configHandler:              new(handler.ConfigHandler),
		sendHandler:                new(handler.SendHandler),
		dataPlaneEmailHandler:      new(handler.DataPlaneEmailHandler),
		externalIntegrationHandler: new(handler.ExternalIntegrationHandler),
		templateTypeHandler:        new(handler.TemplateTypeHandler),
		templateHandler:            new(handler.TemplateHandler),
		injectorHandler:            new(handler.InjectorHandler),
		sesWebhookHandler:          new(handler.SESWebhookHandler),
	})

	srv := sendahttp.NewServer(testServerConfig(), testServerLogger(), opts...)
	routes := routeSet(srv)

	assertRouteRegistered(t, routes, "GET", "/api/v1/manage/tenants")
	assertRouteRegistered(t, routes, "GET", "/api/v1/external/:profile_slug/bootstrap")
	assertRouteRegistered(t, routes, "GET", "/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/template-types")
	assertRouteRegistered(t, routes, "POST", "/api/v1/send")
	assertRouteRegistered(t, routes, "GET", "/api/v1/emails")
	assertRouteRegistered(t, routes, "POST", "/api/v1/webhooks/ses/inbound")
}

func TestNewServerOptions_OmitsSESWebhookSurfaceWhenHandlerMissing(t *testing.T) {
	opts := newServerOptions(serverSharedDeps{}, serverHandlerBundle{})

	srv := sendahttp.NewServer(testServerConfig(), testServerLogger(), opts...)
	routes := routeSet(srv)

	if _, ok := routes["POST /api/v1/webhooks/ses/inbound"]; ok {
		t.Fatal("expected SES webhook route to be omitted when handler bundle does not provide it")
	}
}

func testServerConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}
}

func testServerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func routeSet(srv *sendahttp.Server) map[string]struct{} {
	routes := make(map[string]struct{})
	for _, route := range srv.Echo().Router().Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	return routes
}

func assertRouteRegistered(t *testing.T, routes map[string]struct{}, method, path string) {
	t.Helper()

	if _, ok := routes[method+" "+path]; !ok {
		t.Fatalf("expected route %s %s to be registered", method, path)
	}
}

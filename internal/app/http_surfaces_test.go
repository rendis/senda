package app

import (
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/rendis/senda/config"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
)

func TestManagementSurfaceOptions_RegisterOnlyManagementRoutes(t *testing.T) {
	srv := sendahttp.NewServer(testSurfaceConfig(), testSurfaceLogger(), managementSurfaceOptions(managementSurfaceHandlers{
		config: handler.NewConfigHandler(nil, handler.OIDCInfo{}),
	})...)

	if !appRouteExists(srv, http.MethodGet, "/api/v1/manage/config") {
		t.Fatalf("expected management config route to be registered")
	}
	if appRouteExists(srv, http.MethodPost, "/api/v1/send") {
		t.Fatalf("did not expect data-plane route in management-only surface")
	}
	if appRouteExists(srv, http.MethodGet, "/api/v1/external/:profile_slug/bootstrap") {
		t.Fatalf("did not expect external route in management-only surface")
	}
}

func TestDataPlaneSurfaceOptions_RegisterOnlyDataPlaneRoutes(t *testing.T) {
	srv := sendahttp.NewServer(testSurfaceConfig(), testSurfaceLogger(), dataPlaneSurfaceOptions(dataPlaneSurfaceHandlers{
		send:           handler.NewSendHandler(nil, 10),
		dataPlaneEmail: handler.NewDataPlaneEmailHandler(nil),
	})...)

	if !appRouteExists(srv, http.MethodPost, "/api/v1/send") {
		t.Fatalf("expected data-plane send route to be registered")
	}
	if appRouteExists(srv, http.MethodGet, "/api/v1/manage/config") {
		t.Fatalf("did not expect management route in data-plane-only surface")
	}
}

func TestExternalIntegrationSurfaceOptions_RegisterOnlyExternalRoutes(t *testing.T) {
	srv := sendahttp.NewServer(testSurfaceConfig(), testSurfaceLogger(), externalIntegrationSurfaceOptions(externalIntegrationSurfaceHandlers{
		externalIntegration: handler.NewExternalIntegrationHandler(nil, nil, nil),
	})...)

	if !appRouteExists(srv, http.MethodGet, "/api/v1/external/:profile_slug/bootstrap") {
		t.Fatalf("expected external bootstrap route to be registered")
	}
	if appRouteExists(srv, http.MethodPost, "/api/v1/send") {
		t.Fatalf("did not expect data-plane route in external-only surface")
	}
}

func TestPublicSurfaceOptions_RegisterOnlyPublicRoutes(t *testing.T) {
	srv := sendahttp.NewServer(testSurfaceConfig(), testSurfaceLogger(), publicSurfaceOptions(publicSurfaceHandlers{
		tracking: &handler.TrackingHandler{},
		media:    &handler.MediaHandler{},
	})...)

	if !appRouteExists(srv, http.MethodGet, "/t/o/:tracking_id") {
		t.Fatalf("expected public tracking route to be registered")
	}
	if !appRouteExists(srv, http.MethodGet, "/public/video-thumbnail") {
		t.Fatalf("expected public media thumbnail route to be registered")
	}
	if appRouteExists(srv, http.MethodPost, "/api/v1/send") {
		t.Fatalf("did not expect data-plane route in public-only surface")
	}
	if appRouteExists(srv, http.MethodGet, "/api/v1/manage/config") {
		t.Fatalf("did not expect management route in public-only surface")
	}
}

func testSurfaceConfig() *config.Config {
	return &config.Config{Server: config.ServerConfig{Host: "0.0.0.0", Port: 8080}}
}

func testSurfaceLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func appRouteExists(srv *sendahttp.Server, method, path string) bool {
	for _, route := range srv.Echo().Router().Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	httpmw "github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/sdk"
)

func main() {
	cfgPath := os.Getenv("SENDA_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	engine := sdk.NewWithConfig(cfgPath)
	if os.Getenv("SENDA_E2E_ENABLE_CODE_INJECTORS") == "true" {
		slog.Info("registering e2e code injectors")
		for _, reg := range demoCodeInjectors() {
			engine.RegisterInjector(reg)
		}
	} else {
		slog.Warn("e2e code injectors disabled")
	}
	if os.Getenv("SENDA_E2E_ENABLE_EXTERNAL_INTEGRATION") == "true" {
		slog.Info("registering e2e external integration methods")
		engine.RegisterExternalAuthMethod(e2eExternalAuthMethod{
			token: externalToken(),
		})
		engine.RegisterExternalWorkspaceResolver(e2eExternalWorkspaceResolver{})
	} else {
		slog.Warn("e2e external integration disabled")
	}

	if err := engine.Run(); err != nil {
		slog.Error("senda e2e failed", "error", err)
		os.Exit(1)
	}
}

type e2eExternalAuthMethod struct {
	token string
}

func (e e2eExternalAuthMethod) Name() string { return "e2e-signed-token" }

func (e e2eExternalAuthMethod) Description() string {
	return "E2E token auth for external integration tests"
}

func (e e2eExternalAuthMethod) Authenticate(_ context.Context, req *sdk.ExternalIntegrationRequest) (*sdk.ExternalAuthResult, error) {
	if req == nil {
		return nil, errors.New("missing external integration request")
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		token = strings.TrimSpace(req.QueryParams["token"])
	}
	if token == "" {
		token = strings.TrimSpace(req.Headers["x-senda-external-token"])
	}
	if tenantHeader := strings.TrimSpace(req.Headers["x-tenant-code"]); tenantHeader != "" && !strings.EqualFold(tenantHeader, req.TenantCode) {
		return nil, httpmw.ExternalAuthDenied()
	}

	permissions := sdk.ExternalPermissions{
		ListTemplates:   true,
		ViewVersions:    true,
		EditVersions:    true,
		PublishVersions: true,
		TestSend:        true,
		BuilderAccess:   true,
		MetadataAccess:  true,
		LocaleAccess:    true,
	}

	switch token {
	case "external-e2e-viewer":
		permissions = sdk.ExternalPermissions{
			ListTemplates:   true,
			ViewVersions:    true,
			EditVersions:    false,
			PublishVersions: false,
			TestSend:        false,
			BuilderAccess:   true,
			MetadataAccess:  true,
			LocaleAccess:    true,
		}
	case e.token:
	default:
		return nil, httpmw.ExternalAuthDenied()
	}

	return &sdk.ExternalAuthResult{
		Permissions: permissions,
		Context: map[string]any{
			"tenant_code": req.TenantCode,
			"token":       token,
		},
	}, nil
}

type e2eExternalWorkspaceResolver struct{}

func (e2eExternalWorkspaceResolver) Name() string { return "e2e-workspace-resolver" }

func (e2eExternalWorkspaceResolver) Description() string {
	return "E2E resolver that can force workspace fallback via query params"
}

func (e2eExternalWorkspaceResolver) ResolveWorkspace(_ context.Context, req *sdk.ExternalIntegrationRequest, _ *sdk.ExternalAuthResult) (*sdk.ExternalWorkspaceResolution, error) {
	if req == nil {
		return nil, errors.New("missing external integration request")
	}

	if req.QueryParams["fallback"] == "system" {
		return &sdk.ExternalWorkspaceResolution{
			WorkspaceCode: "_system",
			ReadOnly:      true,
		}, nil
	}

	if len(req.WorkspaceCodes) == 0 || strings.TrimSpace(req.WorkspaceCodes[0]) == "" {
		return nil, errors.New("missing workspace code")
	}

	return &sdk.ExternalWorkspaceResolution{
		WorkspaceCode: strings.TrimSpace(req.WorkspaceCodes[0]),
		ReadOnly:      false,
	}, nil
}

func externalToken() string {
	if token := strings.TrimSpace(os.Getenv("SENDA_E2E_EXTERNAL_TOKEN")); token != "" {
		return token
	}
	return "external-e2e-token"
}

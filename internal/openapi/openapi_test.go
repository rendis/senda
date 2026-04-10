package openapi

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestNormalizeEchoPath(t *testing.T) {
	t.Parallel()

	got := NormalizeEchoPath("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/templates/:template_id")
	want := "/api/v1/manage/tenants/{tenant_code}/workspaces/{workspace_code}/templates/{template_id}"

	if got != want {
		t.Fatalf("NormalizeEchoPath() = %q, want %q", got, want)
	}
}

func TestRouteSecurity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want []string
	}{
		{path: "/health", want: nil},
		{path: "/api/v1/send", want: []string{"WorkspaceAPIKeyBearer"}},
		{path: "/api/v1/send/batch", want: []string{"WorkspaceAPIKeyBearer"}},
		{path: "/api/v1/emails/:tracking_id", want: []string{"WorkspaceAPIKeyBearer"}},
		{path: "/api/v1/onboarding/setup", want: []string{"ManagementBearer"}},
		{path: "/api/v1/members/me", want: []string{"ManagementBearer"}},
		{path: "/api/v1/manage/config", want: []string{"ManagementBearer"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			got := routeSecurity(Route{Path: tt.path})
			if len(got) != len(tt.want) {
				t.Fatalf("routeSecurity(%q) len = %d, want %d", tt.path, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("routeSecurity(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRouteParametersAddsExternalEnvironmentHeader(t *testing.T) {
	t.Parallel()

	got := routeParameters(Route{
		Method:     "GET",
		Path:       "/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/session",
		Parameters: []string{"profile_slug", "tenant_code", "workspace_code"},
	})

	var found bool
	for _, param := range got {
		if param.Name == "X-Senda-Environment" && param.In == "header" && param.Required {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("routeParameters() did not include required X-Senda-Environment header: %#v", got)
	}
}

func TestPatchSecuritySchemes(t *testing.T) {
	t.Parallel()

	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{},
			SecuritySchemes: openapi3.SecuritySchemes{
				"ManagementBearer":      &openapi3.SecuritySchemeRef{Value: &openapi3.SecurityScheme{Type: "apiKey"}},
				"WorkspaceAPIKeyBearer": &openapi3.SecuritySchemeRef{Value: &openapi3.SecurityScheme{Type: "apiKey"}},
			},
		},
	}

	PatchSecuritySchemes(doc)

	mgmt := doc.Components.SecuritySchemes["ManagementBearer"].Value
	if mgmt.Type != "http" || mgmt.Scheme != "bearer" || mgmt.BearerFormat != "JWT" {
		t.Fatalf("management scheme = %#v, want http bearer JWT", mgmt)
	}

	data := doc.Components.SecuritySchemes["WorkspaceAPIKeyBearer"].Value
	if data.Type != "http" || data.Scheme != "bearer" || data.BearerFormat != "senda_prod|senda_test" {
		t.Fatalf("workspace API key scheme = %#v, want http bearer senda_prod|senda_test", data)
	}
}

func TestValidateRouteCoverage(t *testing.T) {
	t.Parallel()

	doc := &openapi3.T{
		Paths: openapi3.NewPaths(),
	}
	doc.Paths.Set("/health", &openapi3.PathItem{Get: &openapi3.Operation{}})
	doc.Paths.Set("/api/v1/send", &openapi3.PathItem{Post: &openapi3.Operation{}})
	doc.Paths.Set("/api/v1/send/batch", &openapi3.PathItem{Post: &openapi3.Operation{}})

	routes := []Route{
		{Method: "GET", Path: "/health"},
		{Method: "POST", Path: "/api/v1/send"},
		{Method: "POST", Path: "/api/v1/send/batch"},
	}

	if err := ValidateRouteCoverage(doc, routes); err != nil {
		t.Fatalf("ValidateRouteCoverage() unexpected error: %v", err)
	}

	routes = append(routes, Route{Method: "GET", Path: "/metrics"})
	if err := ValidateRouteCoverage(doc, routes); err == nil || !strings.Contains(err.Error(), "missing from OpenAPI") {
		t.Fatalf("ValidateRouteCoverage() error = %v, want missing route error", err)
	}
}

func TestRegisteredRoutesIncludesEnvironmentAndExternalSurfaces(t *testing.T) {
	t.Parallel()

	routes, err := RegisteredRoutes()
	if err != nil {
		t.Fatalf("RegisteredRoutes() error = %v", err)
	}

	wanted := map[string]bool{
		"GET /api/v1/manage/environments/{environment}/tenants/{tenant_code}/workspaces":                                            false,
		"POST /api/v1/manage/environments/{environment}/tenants/{tenant_code}/workspaces/{workspace_code}/runtime/reset":           false,
		"GET /api/v1/manage/environments/{environment}/tenants/{tenant_code}/workspaces/{workspace_code}/policies":                 false,
		"PUT /api/v1/manage/environments/{environment}/tenants/{tenant_code}/workspaces/{workspace_code}/policies":                 false,
		"GET /api/v1/external/{profile_slug}/bootstrap":                                                                              false,
		"GET /api/v1/external/{profile_slug}/environments/{environment}/bootstrap":                                                  false,
		"GET /api/v1/external/{profile_slug}/tenants/{tenant_code}/workspaces/{workspace_code}/session":                            false,
	}

	for _, route := range routes {
		key := route.Method + " " + NormalizeEchoPath(route.Path)
		if _, ok := wanted[key]; ok {
			wanted[key] = true
		}
	}

	for key, found := range wanted {
		if !found {
			t.Fatalf("RegisteredRoutes() missing %s", key)
		}
	}
}

func TestGenerateSwagDocsContent(t *testing.T) {
	t.Parallel()

	content := GenerateSwagDocsContent([]Route{
		{Method: "POST", Path: "/api/v1/send", Parameters: nil},
		{Method: "GET", Path: "/api/v1/manage/tenants/:tenant_code", Parameters: []string{"tenant_code"}},
		{Method: "GET", Path: "/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/session", Parameters: []string{"profile_slug", "tenant_code", "workspace_code"}},
	})

	for _, want := range []string{
		"package main",
		"@Router       /api/v1/send [post]",
		"@Security     WorkspaceAPIKeyBearer",
		"@Router       /api/v1/manage/tenants/{tenant_code} [get]",
		"@Param        tenant_code  path",
		"@Security     ManagementBearer",
		"@Router       /api/v1/external/{profile_slug}/tenants/{tenant_code}/workspaces/{workspace_code}/session [get]",
		"@Param        X-Senda-Environment  header  string  true",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("generated docs missing %q\n%s", want, content)
		}
	}
}

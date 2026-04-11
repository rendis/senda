package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/sdk"
)

func TestDemoCodeInjectorsIncludeStaticCatalogExamples(t *testing.T) {
	regs := demoCodeInjectors()
	byCode := map[string]sdk.InjectorRegistration{}
	for _, reg := range regs {
		byCode[reg.Code] = reg
	}

	brand, ok := byCode["brand"]
	if !ok {
		t.Fatalf("brand injector not registered")
	}
	if !brand.Static {
		t.Fatalf("brand injector should be static")
	}
	if brand.TTL != 2*time.Minute {
		t.Fatalf("brand TTL = %s, want %s", brand.TTL, 2*time.Minute)
	}
	if len(brand.Fields) < 4 {
		t.Fatalf("brand fields = %d, want >= 4", len(brand.Fields))
	}

	workspaceProfile, ok := byCode["workspace_profile"]
	if !ok {
		t.Fatalf("workspace_profile injector not registered")
	}
	if !workspaceProfile.Static {
		t.Fatalf("workspace_profile should be static")
	}

	student, ok := byCode["student"]
	if !ok {
		t.Fatalf("student injector not registered")
	}
	if student.Static {
		t.Fatalf("student injector should be dynamic/runtime-only")
	}
}

func TestBrandInjectorResolveReturnsRenderableHtmlAndWorkspaceAwareText(t *testing.T) {
	reg := demoBrandInjector()
	resolve, _ := reg.Resolve, reg.Dependencies
	ctx := sdk.NewInjectorContext(nil, "system-test-corp:system-main:welcome-email", nil, uuid.New(), uuid.New(), domain.EnvironmentProd, "welcome-email")

	values, err := resolve(context.Background(), ctx)
	if err != nil {
		t.Fatalf("resolve brand injector: %v", err)
	}

	if got := values["company_name"]; got != "System Main Academy" {
		t.Fatalf("company_name = %v, want %q", got, "System Main Academy")
	}
	footer, _ := values["footer_html"].(string)
	if !strings.Contains(footer, "<strong>System Main Academy</strong>") {
		t.Fatalf("footer_html missing branded html: %q", footer)
	}
	if !strings.Contains(footer, "https://system-main.demo.senda.test/policies/email") {
		t.Fatalf("footer_html missing policy link: %q", footer)
	}
}

func TestWorkspaceProfileInjectorVariesByWorkspaceAndEnvironment(t *testing.T) {
	reg := demoWorkspaceProfileInjector()
	ctx := sdk.NewInjectorContext(nil, "system-test-corp:campus-north:welcome-email", nil, uuid.New(), uuid.New(), domain.EnvironmentTest, "welcome-email")

	values, err := reg.Resolve(context.Background(), ctx)
	if err != nil {
		t.Fatalf("resolve workspace_profile injector: %v", err)
	}

	if got := values["workspace_label"]; got != "Campus North" {
		t.Fatalf("workspace_label = %v, want %q", got, "Campus North")
	}
	badge, _ := values["environment_badge_html"].(string)
	if !strings.Contains(badge, "TEST") {
		t.Fatalf("environment badge should mention TEST, got %q", badge)
	}
}

func TestStudentInjectorUsesRequestOverridesWhenProvided(t *testing.T) {
	reg := demoStudentInjector()
	ctx := sdk.NewInjectorContext(nil, "system-test-corp:system-main:welcome-email", nil, uuid.New(), uuid.New(), domain.EnvironmentProd, "welcome-email")
	ctx.SetRequestInjectors(map[string]map[string]any{
		"student": {
			"name":   "Ada Lovelace",
			"status": "override-status",
		},
	})

	values, err := reg.Resolve(context.Background(), ctx)
	if err != nil {
		t.Fatalf("resolve student injector: %v", err)
	}

	if got := values["name"]; got != "Ada Lovelace" {
		t.Fatalf("name = %v, want %q", got, "Ada Lovelace")
	}
	if got := values["status"]; got != "override-status" {
		t.Fatalf("status = %v, want %q", got, "override-status")
	}
}

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rendis/senda/sdk"
)

func demoCodeInjectors() []sdk.InjectorRegistration {
	return []sdk.InjectorRegistration{
		demoBrandInjector(),
		demoWorkspaceProfileInjector(),
		demoStudentInjector(),
		demoRequestDebugInjector(),
	}
}

func demoBrandInjector() sdk.InjectorRegistration {
	return sdk.InjectorRegistration{
		Code:        "brand",
		Name:        "Brand",
		Description: "Static brand assets and legal/footer content for the active workspace.",
		Static:      true,
		TTL:         2 * time.Minute,
		Fields: []sdk.InjectorFieldSpec{
			{Name: "company_name", Type: sdk.FieldTypeText, Description: "Brand/company display name"},
			{Name: "policy_url", Type: sdk.FieldTypeURL, Description: "Policy/help center URL"},
			{Name: "logo_url", Type: sdk.FieldTypeImg, Description: "Logo image URL"},
			{Name: "hero_image_url", Type: sdk.FieldTypeImg, Description: "Hero image URL"},
			{Name: "footer_html", Type: sdk.FieldTypeHTML, Description: "Renderable HTML footer block"},
		},
		Resolve: func(_ context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
			workspaceCode := workspaceCodeFromRef(injCtx.Ref())
			profile := brandProfileForWorkspace(workspaceCode)
			return map[string]any{
				"company_name":   profile.CompanyName,
				"policy_url":     profile.PolicyURL,
				"logo_url":       profile.LogoURL,
				"hero_image_url": profile.HeroImageURL,
				"footer_html":    profile.FooterHTML,
			}, nil
		},
		Critical: true,
	}
}

func demoWorkspaceProfileInjector() sdk.InjectorRegistration {
	return sdk.InjectorRegistration{
		Code:        "workspace_profile",
		Name:        "Workspace Profile",
		Description: "Static workspace/environment labels rendered directly by the backend.",
		Static:      true,
		TTL:         time.Minute,
		Fields: []sdk.InjectorFieldSpec{
			{Name: "workspace_code", Type: sdk.FieldTypeText, Description: "Resolved workspace code"},
			{Name: "workspace_label", Type: sdk.FieldTypeText, Description: "Human-friendly workspace label"},
			{Name: "environment_name", Type: sdk.FieldTypeText, Description: "Resolved environment name"},
			{Name: "environment_badge_html", Type: sdk.FieldTypeHTML, Description: "Renderable environment badge"},
		},
		Resolve: func(_ context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
			workspaceCode := workspaceCodeFromRef(injCtx.Ref())
			workspaceLabel := humanizeWorkspaceCode(workspaceCode)
			envName := strings.ToUpper(string(injCtx.Environment()))
			return map[string]any{
				"workspace_code":   workspaceCode,
				"workspace_label":  workspaceLabel,
				"environment_name": envName,
				"environment_badge_html": fmt.Sprintf(
					`<span style="display:inline-block;padding:2px 8px;border-radius:999px;background:%s;color:#fff;font-size:12px;font-weight:600;">%s · %s</span>`,
					environmentBadgeColor(injCtx.Environment()),
					envName,
					workspaceLabel,
				),
			}, nil
		},
		Critical: true,
	}
}

func demoStudentInjector() sdk.InjectorRegistration {
	return sdk.InjectorRegistration{
		Code:        "student",
		Description: "Dynamic runtime injector that can be overridden per request.",
		Resolve: func(_ context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
			workspaceCode := workspaceCodeFromRef(injCtx.Ref())
			name := overrideString(injCtx, "student", "name", defaultStudentName(workspaceCode))
			status := overrideString(injCtx, "student", "status", defaultStudentStatus(injCtx.Environment()))
			cohort := overrideString(injCtx, "student", "cohort", fmt.Sprintf("%s-%s", workspaceCode, strings.ToLower(string(injCtx.Environment()))))
			return map[string]any{
				"name":   name,
				"status": status,
				"cohort": cohort,
				"age":    22,
			}, nil
		},
		Critical: true,
	}
}

func demoRequestDebugInjector() sdk.InjectorRegistration {
	return sdk.InjectorRegistration{
		Code:        "request_debug",
		Description: "Dynamic runtime-only injector that exposes request/workspace/environment context.",
		Resolve: func(_ context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
			workspaceCode := workspaceCodeFromRef(injCtx.Ref())
			userName, _ := injCtx.Variables()["user_name"].(string)
			return map[string]any{
				"workspace_code":  workspaceCode,
				"environment":     strings.ToUpper(string(injCtx.Environment())),
				"event_user_name": userName,
				"request_note":    overrideString(injCtx, "request_debug", "request_note", "no-request-note"),
			}, nil
		},
		Critical: false,
	}
}

type brandProfile struct {
	CompanyName  string
	PolicyURL    string
	LogoURL      string
	HeroImageURL string
	FooterHTML   string
}

func brandProfileForWorkspace(workspaceCode string) brandProfile {
	label := humanizeWorkspaceCode(workspaceCode)
	if label == "" {
		label = "Workspace Demo"
	}
	base := fmt.Sprintf("https://%s.demo.senda.test", workspaceCode)
	return brandProfile{
		CompanyName:  label + " Academy",
		PolicyURL:    base + "/policies/email",
		LogoURL:      base + "/assets/logo.png",
		HeroImageURL: base + "/assets/hero.png",
		FooterHTML: fmt.Sprintf(
			`<p style="margin:0;"><strong>%s</strong> · <a href=%q>Email policy</a></p>`,
			label+" Academy",
			base+"/policies/email",
		),
	}
}

func workspaceCodeFromRef(ref string) string {
	parts := strings.Split(ref, ":")
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return "unknown-workspace"
	}
	return strings.TrimSpace(parts[1])
}

func humanizeWorkspaceCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "Unknown Workspace"
	}
	parts := strings.FieldsFunc(code, func(r rune) bool {
		return r == '-' || r == '_' || r == ':'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func environmentBadgeColor(env sdk.Environment) string {
	if env == sdk.EnvironmentTest {
		return "#D97706"
	}
	return "#0F766E"
}

func defaultStudentName(workspaceCode string) string {
	if workspaceCode == "campus-north" {
		return "North Campus Student"
	}
	return "Code Student"
}

func defaultStudentStatus(env sdk.Environment) string {
	if env == sdk.EnvironmentTest {
		return "test-mode-status"
	}
	return "code-status"
}

func overrideString(injCtx *sdk.InjectorContext, injectorCode, fieldName, fallback string) string {
	if injCtx == nil {
		return fallback
	}
	requestValues := injCtx.RequestInjectors()
	fields, ok := requestValues[injectorCode]
	if !ok {
		return fallback
	}
	value, ok := fields[fieldName]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

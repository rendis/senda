package domain

import "testing"

func TestExternalIntegrationProfile_Validate(t *testing.T) {
	profile := ExternalIntegrationProfile{
		Slug:            "partner-portal",
		Name:            "Partner Portal",
		Description:     "Integration for partner-facing UI",
		Enabled:         true,
		AuthMethodName:  "signed-headers",
		ResolverName:    "tenant-workspace-resolver",
		AllowedOrigins:  []string{"https://app.example.com"},
		AllowedHeaders:  []string{"X-Tenant-Code", "X-Trace-ID"},
		RequiredHeaders: []string{"x-tenant-code"},
	}

	if err := profile.Validate(); err != nil {
		t.Fatalf("expected profile to validate, got error: %v", err)
	}
}

func TestExternalIntegrationProfile_Validate_RequiredHeadersSubset(t *testing.T) {
	profile := ExternalIntegrationProfile{
		Slug:            "partner-portal",
		Name:            "Partner Portal",
		Description:     "Integration for partner-facing UI",
		Enabled:         true,
		AuthMethodName:  "signed-headers",
		ResolverName:    "tenant-workspace-resolver",
		AllowedHeaders:  []string{"x-trace-id"},
		RequiredHeaders: []string{"x-tenant-code"},
	}

	if err := profile.Validate(); err == nil {
		t.Fatal("expected validation to fail when required headers are not included in allowed headers")
	}
}

func TestExternalIntegrationProfile_Validate_RejectsInvalidAllowedOrigin(t *testing.T) {
	profile := ExternalIntegrationProfile{
		Slug:           "partner-portal",
		Name:           "Partner Portal",
		Description:    "Integration for partner-facing UI",
		Enabled:        true,
		AuthMethodName: "signed-headers",
		ResolverName:   "tenant-workspace-resolver",
		AllowedOrigins: []string{"https://app.example.com/path"},
	}

	if err := profile.Validate(); err == nil {
		t.Fatal("expected validation to fail when allowed origin is not a bare origin")
	}
}

func TestExternalIntegrationProfile_Validate_RejectsInvalidHeaderName(t *testing.T) {
	profile := ExternalIntegrationProfile{
		Slug:           "partner-portal",
		Name:           "Partner Portal",
		Description:    "Integration for partner-facing UI",
		Enabled:        true,
		AuthMethodName: "signed-headers",
		ResolverName:   "tenant-workspace-resolver",
		AllowedHeaders: []string{"x bad header"},
	}

	if err := profile.Validate(); err == nil {
		t.Fatal("expected validation to fail for invalid header name")
	}
}

func TestExternalIntegrationProfile_Validate_RejectsInvalidMethodNames(t *testing.T) {
	profile := ExternalIntegrationProfile{
		Slug:           "partner-portal",
		Name:           "Partner Portal",
		Description:    "Integration for partner-facing UI",
		Enabled:        true,
		AuthMethodName: "signed headers",
		ResolverName:   "tenant-workspace-resolver",
	}

	if err := profile.Validate(); err == nil {
		t.Fatal("expected validation to fail for invalid auth method name")
	}
}

func TestGlobalConfig_Validate_ExternalProfiles(t *testing.T) {
	cfg := &GlobalConfig{
		ExternalIntegrations: []ExternalIntegrationProfile{
			{
				Slug:           "partner-portal",
				Name:           "Partner Portal",
				Description:    "Integration for partner-facing UI",
				Enabled:        true,
				AuthMethodName: "signed-headers",
				ResolverName:   "tenant-workspace-resolver",
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to validate, got error: %v", err)
	}
}

func TestGlobalConfig_Validate_RejectsDuplicateExternalProfileSlugs(t *testing.T) {
	cfg := &GlobalConfig{
		ExternalIntegrations: []ExternalIntegrationProfile{
			{
				Slug:           "partner-portal",
				Name:           "Partner Portal",
				Description:    "Integration for partner-facing UI",
				Enabled:        true,
				AuthMethodName: "signed-headers",
				ResolverName:   "tenant-workspace-resolver",
			},
			{
				Slug:           "partner-portal",
				Name:           "Partner Portal v2",
				Description:    "Duplicate slug",
				Enabled:        true,
				AuthMethodName: "signed-headers",
				ResolverName:   "tenant-workspace-resolver",
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate profile slugs to be rejected")
	}
}

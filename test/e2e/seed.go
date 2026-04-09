//go:build e2e

package e2e

// Seed data constants for E2E tests.
// These values are used consistently across all test flows.

const (
	// Tenant credentials
	TenantCode = "test-corp"
	TenantName = "Test Corporation"

	// Workspace codes
	WorkspaceCode       = "main"
	WorkspaceName       = "Main Workspace"
	SystemWorkspaceCode = "_system"

	// User emails
	SuperadminEmail      = "superadmin@test.example.com"
	TenantAdminEmail     = "tenant-admin@test.example.com"
	WorkspaceAdminEmail  = "ws-admin@test.example.com"
	WorkspaceEditorEmail = "ws-editor@test.example.com"
	WorkspaceViewerEmail = "ws-viewer@test.example.com"

	// Template resources
	TemplateTypeSlug = "welcome-email"
	TemplateTypeDesc = "Welcome email for new users"
	TemplateSlug     = "welcome-v1"
	TemplateTypeName = "Welcome Email"

	// From address (provider validates identity)
	TestFromEmail = "noreply@mail.test.example.com"
	TestFromName  = "Test Corp"

	// Adapter type — DB only supports "ses" or "gmail"; actual delivery uses bootstrap SMTP adapter.
	AdapterType = "ses"
	AdapterName = "E2E Test Adapter"

	// Shared test harness configuration.
	DefaultMasterKey          = "e2e-master-key-must-be-at-least-32-characters"
	defaultAWSRegion          = "us-east-1"
	defaultAWSAccessKeyID     = "test"
	defaultAWSSecretAccessKey = "test"

	// API Key reference
	APIKeyName       = "test-key"
	APIKeyNamePrefix = "test-key-"

	// Test send content
	TestSubject     = "Welcome to Test Corp"
	TestEmailBody   = "<p>Hello {{first_name}}, welcome to Test Corp!</p>"
	TestPreviewText = "Welcome to Test Corp"

	// Webhook test
	WebhookName = "test-webhook"
	WebhookURL  = "http://webhook.test.example.com/events"
)

// OnboardingRequest is the request structure for the onboarding API.
type OnboardingRequest struct {
	AdminEmail    string `json:"admin_email"`
	AdminName     string `json:"admin_name"`
	TenantCode    string `json:"tenant_code"`
	TenantName    string `json:"tenant_name"`
	WorkspaceCode string `json:"workspace_code"`
	WorkspaceName string `json:"workspace_name"`
}

// TemplateTypeRequest is the request structure for creating a template type.
type TemplateTypeRequest struct {
	Slug           string                 `json:"slug"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	AdapterID      string                 `json:"adapter_id,omitempty"`
	VariableSchema map[string]interface{} `json:"variable_schema,omitempty"`
}

// CreateTemplateRequest is the request structure for creating a template.
type CreateTemplateRequest struct {
	TemplateTypeID   string `json:"template_type_id,omitempty"`
	TemplateTypeSlug string `json:"template_type_slug,omitempty"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Description      string `json:"description"`
}

// CreateVersionRequest is the request structure for creating a template version.
type CreateVersionRequest struct {
	Subject       string `json:"subject"`
	PreviewText   string `json:"preview_text"`
	FromEmail     string `json:"from_email"`
	FromName      string `json:"from_name"`
	BodyMJML      string `json:"body_mjml"`
	DefaultLocale string `json:"default_locale"`
}

// CreateLocaleRequest is the request structure for creating a locale.
type CreateLocaleRequest struct {
	Locale      string `json:"locale"`
	Subject     string `json:"subject"`
	PreviewText string `json:"preview_text"`
	FromEmail   string `json:"from_email"`
	FromName    string `json:"from_name"`
	BodyMJML    string `json:"body_mjml"`
}

// SendRequest is the request structure for sending an email.
type SendRequest struct {
	Ref        string                 `json:"ref"`
	To         []string               `json:"to"`
	CC         []string               `json:"cc,omitempty"`
	BCC        []string               `json:"bcc,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	Injectors  map[string]map[string]interface{} `json:"injectors,omitempty"`
	ExternalID string                 `json:"external_id,omitempty"`
	Locale     string                 `json:"locale,omitempty"`
}

// SendBatchRequest is the request structure for batch sending an email.
type SendBatchRequest struct {
	Ref   string                 `json:"ref"`
	Items []SendBatchItemRequest `json:"items"`
}

// SendBatchItemRequest is one logical message inside a batch request.
type SendBatchItemRequest struct {
	To         string                 `json:"to"`
	CC         []string               `json:"cc,omitempty"`
	BCC        []string               `json:"bcc,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	Injectors  map[string]map[string]interface{} `json:"injectors,omitempty"`
	ExternalID string                 `json:"external_id,omitempty"`
	Locale     string                 `json:"locale,omitempty"`
}

// InjectorRequest is the request structure for creating an injector.
type InjectorRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Fields      []InjectorFieldRequest `json:"fields"`
}

// InjectorFieldRequest defines a field in an injector schema.
type InjectorFieldRequest struct {
	FieldName      string      `json:"field_name"`
	FieldType      string      `json:"field_type"`
	Description    string      `json:"description,omitempty"`
	Position       int         `json:"position,omitempty"`
	DefaultValue   interface{} `json:"default_value,omitempty"`
	AllowOverwrite *bool       `json:"allow_overwrite,omitempty"`
}

// SetInjectorValuesRequest sets injector field values.
type SetInjectorValuesRequest struct {
	Values []InjectorFieldValue `json:"values"`
}

// InjectorFieldValue maps a field name to its value.
type InjectorFieldValue struct {
	FieldName string `json:"field_name"`
	Value     string `json:"value"`
}

// AdapterRequest is the request structure for creating an adapter.
type AdapterRequest struct {
	Name               string                 `json:"name"`
	AdapterType        string                 `json:"adapter_type"`
	Config             map[string]interface{} `json:"config"`
	IsDefault          bool                   `json:"is_default,omitempty"`
	RateLimitPerSecond int                    `json:"rate_limit_per_second,omitempty"`
}

// APIKeyRequest is the request structure for creating an API key.
type APIKeyRequest struct {
	Name string `json:"name"`
}

// MemberRequest is the request structure for inviting a member.
type MemberRequest struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// WebhookRequest is the request structure for creating a webhook.
type WebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// SuppressionRequest is the request structure for adding to suppression list.
type SuppressionRequest struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

// SampleMJML returns a basic MJML template for testing.
func SampleMJML() string {
	return `<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text font-size="20px" align="center" color="#1f4788">
          Welcome {{ event.first_name }}!
        </mj-text>
        <mj-text align="center" color="#626262">
          Thank you for joining {{ event.company_name }}.
        </mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`
}

// DefaultVariableSchema returns the default template variable schema for testing.
func DefaultVariableSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"first_name": map[string]interface{}{
				"type": "string",
			},
			"company_name": map[string]interface{}{
				"type": "string",
			},
		},
	}
}

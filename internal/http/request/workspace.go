package request

// CreateWorkspaceRequest is the request body for POST /api/v1/manage/tenants/:tenant_code/workspaces.
type CreateWorkspaceRequest struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	DefaultLocale *string `json:"default_locale"`
}

// UpdateWorkspaceRequest is the request body for PUT /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code.
type UpdateWorkspaceRequest struct {
	Code                *string `json:"code,omitempty"`
	Name                *string `json:"name"`
	IsActive            *bool   `json:"is_active,omitempty"`
	OpenTrackingEnabled *bool   `json:"open_tracking_enabled"`
	DefaultLocale       *string `json:"default_locale"`
	TestRecipientMode   *string   `json:"test_recipient_mode,omitempty"`
	TestRecipientAddresses *[]string `json:"test_recipient_addresses,omitempty"`
}

// UpdateWorkspacePolicyRequest is the request body for PUT /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/policies.
type UpdateWorkspacePolicyRequest struct {
	AllowWorkspaceLocalTemplates         *bool `json:"allow_workspace_local_templates"`
	AllowWorkspaceInheritedTemplateForks *bool `json:"allow_workspace_inherited_template_forks"`
	AllowWorkspaceLocalInjectors         *bool `json:"allow_workspace_local_injectors"`
}

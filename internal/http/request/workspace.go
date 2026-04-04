package request

// CreateWorkspaceRequest is the request body for POST /api/v1/manage/tenants/:tenant_code/workspaces.
type CreateWorkspaceRequest struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	DefaultLocale *string `json:"default_locale"`
}

// UpdateWorkspaceRequest is the request body for PUT /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code.
type UpdateWorkspaceRequest struct {
	Name                *string `json:"name"`
	IsActive            *bool   `json:"is_active,omitempty"`
	OpenTrackingEnabled *bool   `json:"open_tracking_enabled"`
	DefaultLocale       *string `json:"default_locale"`
}

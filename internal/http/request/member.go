package request

// CreateMemberRequest is the request body for POST /api/v1/manage/members.
type CreateMemberRequest struct {
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name"`
}

// AddRoleRequest is the request body for POST /api/v1/manage/members/:member_id/roles.
type AddRoleRequest struct {
	Role        string  `json:"role"`
	ScopeType   string  `json:"scope_type"`
	TenantID    *string `json:"tenant_id"`
	WorkspaceID *string `json:"workspace_id"`
}

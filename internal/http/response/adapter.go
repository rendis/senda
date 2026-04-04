package response

import (
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// AdapterResponse is the JSON response for an adapter.
// SECURITY: ConfigEncrypted is never exposed. Only non-sensitive metadata is shown.
type AdapterResponse struct {
	ID                 string            `json:"id"`
	WorkspaceID        *string           `json:"workspace_id,omitempty"`
	SourceScope        *string           `json:"source_scope,omitempty"`
	SourceWorkspaceID  *string           `json:"source_workspace_id,omitempty"`
	Name               string            `json:"name"`
	AdapterType        string            `json:"adapter_type"`
	IsDefault          bool              `json:"is_default"`
	IsEditable         bool              `json:"is_editable"`
	IsShared           bool              `json:"is_shared"`
	RateLimitPerSecond int               `json:"rate_limit_per_second"`
	ConfigMeta         map[string]string `json:"config_meta,omitempty"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
}

// AdapterListResponse is the JSON response for a paginated list of adapters.
type AdapterListResponse struct {
	Items      []AdapterResponse `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
	HasMore    bool              `json:"has_more"`
}

// NewAdapterResponse maps a domain Adapter to an AdapterResponse.
func NewAdapterResponse(a *domain.Adapter) AdapterResponse {
	resp := AdapterResponse{
		ID:                 a.ID.String(),
		Name:               a.Name,
		AdapterType:        string(a.AdapterType),
		IsDefault:          a.IsDefault,
		IsEditable:         true,
		RateLimitPerSecond: a.RateLimitPerSecond,
		ConfigMeta:         a.ConfigMeta,
		CreatedAt:          a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:          a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if a.WorkspaceID != nil {
		s := a.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	return resp
}

// NewAdapterResponseForWorkspace maps a domain Adapter to a workspace-aware response, including sharing metadata.
func NewAdapterResponseForWorkspace(a *domain.Adapter, workspace *domain.Workspace) AdapterResponse {
	resp := NewAdapterResponse(a)
	if workspace == nil {
		return resp
	}
	if a.WorkspaceID != nil {
		sourceID := a.WorkspaceID.String()
		resp.SourceWorkspaceID = &sourceID
	}
	switch {
	case workspace.IsSystem:
		sourceScope := "system"
		resp.SourceScope = &sourceScope
		resp.IsEditable = true
		resp.IsShared = false
	case a.WorkspaceID != nil && *a.WorkspaceID == workspace.ID:
		sourceScope := "workspace"
		resp.SourceScope = &sourceScope
		resp.IsEditable = true
		resp.IsShared = false
	case a.WorkspaceID != nil:
		sourceScope := "system"
		resp.SourceScope = &sourceScope
		resp.IsEditable = false
		resp.IsShared = true
	default:
		resp.IsEditable = false
		resp.IsShared = false
	}
	return resp
}

// NewAdapterListResponse maps a PageResult of adapters to an AdapterListResponse.
func NewAdapterListResponse(page *port.PageResult[domain.Adapter]) AdapterListResponse {
	items := make([]AdapterResponse, len(page.Items))
	for i, a := range page.Items {
		items[i] = NewAdapterResponse(a)
	}
	return AdapterListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}

// WorkspaceAccessItemResponse is the JSON response row for adapter/identity workspace sharing.
type WorkspaceAccessItemResponse struct {
	WorkspaceID string `json:"workspace_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	IsGranted   bool   `json:"is_granted"`
}

// WorkspaceAccessListResponse is the JSON response for workspace sharing toggles.
type WorkspaceAccessListResponse struct {
	Items []WorkspaceAccessItemResponse `json:"items"`
}

// NewWorkspaceAccessListResponse maps service grants to a response.
func NewWorkspaceAccessListResponse(grants []service.WorkspaceAccessGrant) WorkspaceAccessListResponse {
	items := make([]WorkspaceAccessItemResponse, len(grants))
	for i, grant := range grants {
		items[i] = WorkspaceAccessItemResponse{
			WorkspaceID: grant.Workspace.ID.String(),
			Code:        grant.Workspace.Code,
			Name:        grant.Workspace.Name,
			IsGranted:   grant.Granted,
		}
	}
	return WorkspaceAccessListResponse{Items: items}
}

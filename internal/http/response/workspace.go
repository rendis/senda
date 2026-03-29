package response

import (
	"github.com/rendis/senda/internal/domain"
)

// WorkspaceResponse is the JSON response for a single workspace.
type WorkspaceResponse struct {
	ID                  string  `json:"id"`
	TenantID            string  `json:"tenant_id"`
	Code                string  `json:"code"`
	Name                string  `json:"name"`
	IsSystem            bool    `json:"is_system"`
	OpenTrackingEnabled bool    `json:"open_tracking_enabled"`
	DefaultLocale       *string `json:"default_locale"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

// WorkspaceListResponse is the JSON response for a paginated list of workspaces.
type WorkspaceListResponse struct {
	Items      []WorkspaceResponse `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
	HasMore    bool                `json:"has_more"`
}

// NewWorkspaceResponse maps a domain Workspace to a WorkspaceResponse.
func NewWorkspaceResponse(ws *domain.Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:                  ws.ID.String(),
		TenantID:            ws.TenantID.String(),
		Code:                ws.Code,
		Name:                ws.Name,
		IsSystem:            ws.IsSystem,
		OpenTrackingEnabled: ws.OpenTrackingEnabled,
		DefaultLocale:       ws.DefaultLocale,
		CreatedAt:           ws.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           ws.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

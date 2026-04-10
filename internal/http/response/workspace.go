package response

import (
	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
)

// WorkspaceResponse is the JSON response for a single workspace.
type WorkspaceResponse struct {
	ID                     string   `json:"id"`
	LogicalWorkspaceID     string   `json:"logical_workspace_id"`
	TenantID               string   `json:"tenant_id"`
	Code                   string   `json:"code"`
	Name                   string   `json:"name"`
	Environment            string   `json:"environment"`
	IsSystem               bool     `json:"is_system"`
	IsActive               bool     `json:"is_active"`
	OpenTrackingEnabled    bool     `json:"open_tracking_enabled"`
	DefaultLocale          *string  `json:"default_locale"`
	TestRecipientMode      string   `json:"test_recipient_mode"`
	TestRecipientAddresses []string `json:"test_recipient_addresses"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
}

// WorkspaceListResponse is the JSON response for a paginated list of workspaces.
type WorkspaceListResponse struct {
	Items      []WorkspaceResponse `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
	HasMore    bool                `json:"has_more"`
}

// NewWorkspaceResponse maps a domain Workspace to a WorkspaceResponse.
func NewWorkspaceResponse(ws *domain.Workspace) WorkspaceResponse {
	logicalWorkspaceID := ws.LogicalWorkspaceID
	if logicalWorkspaceID == uuid.Nil {
		logicalWorkspaceID = ws.ID
	}
	environment := ws.Environment
	if !environment.Valid() {
		environment = domain.EnvironmentProd
	}
	testRecipientMode := ws.TestRecipientMode
	if !testRecipientMode.Valid() {
		testRecipientMode = domain.TestRecipientModeReplace
	}
	testRecipientAddresses := append([]string{}, ws.TestRecipientAddresses...)
	return WorkspaceResponse{
		ID:                     ws.ID.String(),
		LogicalWorkspaceID:     logicalWorkspaceID.String(),
		TenantID:               ws.TenantID.String(),
		Code:                   ws.Code,
		Name:                   ws.Name,
		Environment:            environment.String(),
		IsSystem:               ws.IsSystem,
		IsActive:               ws.IsActive,
		OpenTrackingEnabled:    ws.OpenTrackingEnabled,
		DefaultLocale:          ws.DefaultLocale,
		TestRecipientMode:      string(testRecipientMode),
		TestRecipientAddresses: testRecipientAddresses,
		CreatedAt:              ws.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:              ws.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperadmin      Role = "superadmin"
	RoleTenantAdmin     Role = "tenant_admin"
	RoleWorkspaceAdmin  Role = "workspace_admin"
	RoleWorkspaceEditor Role = "workspace_editor"
	RoleWorkspaceViewer Role = "workspace_viewer"
)

// Level returns the role's numeric level for comparison.
// Higher = more permissions.
func (r Role) Level() int {
	switch r {
	case RoleSuperadmin:
		return 100
	case RoleTenantAdmin:
		return 80
	case RoleWorkspaceAdmin:
		return 60
	case RoleWorkspaceEditor:
		return 40
	case RoleWorkspaceViewer:
		return 20
	default:
		return 0
	}
}

type Member struct {
	ID          uuid.UUID
	Email       string
	DisplayName *string
	OIDCSubject *string
	OIDCIssuer  *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MemberRole struct {
	ID          uuid.UUID
	MemberID    uuid.UUID
	Role        Role
	ScopeType   ScopeType
	TenantID    *uuid.UUID
	WorkspaceID *uuid.UUID
	CreatedAt   time.Time
	CreatedBy   *uuid.UUID
}

type ScopeType string

const (
	ScopeGlobal    ScopeType = "global"
	ScopeTenant    ScopeType = "tenant"
	ScopeWorkspace ScopeType = "workspace"
)

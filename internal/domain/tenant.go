package domain

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID        uuid.UUID
	Code      string
	Name      string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type Workspace struct {
	ID                                   uuid.UUID
	LogicalWorkspaceID                   uuid.UUID
	TenantID                             uuid.UUID
	Code                                 string
	Name                                 string
	Environment                          Environment
	IsSystem                             bool
	IsActive                             bool
	OpenTrackingEnabled                  bool
	DefaultLocale                        *string
	TestRecipientMode                    TestRecipientMode
	TestRecipientAddresses               []string
	AllowWorkspaceLocalTemplates         bool
	AllowWorkspaceInheritedTemplateForks bool
	AllowWorkspaceLocalInjectors         bool
	WorkspacePoliciesInitialized         bool
	CreatedAt                            time.Time
	UpdatedAt                            time.Time
	DeletedAt                            *time.Time
}

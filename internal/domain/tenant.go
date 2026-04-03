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
	ID                  uuid.UUID
	TenantID            uuid.UUID
	Code                string
	Name                string
	IsSystem            bool
	OpenTrackingEnabled bool
	DefaultLocale       *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
}

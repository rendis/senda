package domain

import (
	"time"

	"github.com/google/uuid"
)

type IdentityType string

const (
	IdentityTypeEmail  IdentityType = "email"
	IdentityTypeDomain IdentityType = "domain"
)

type IdentityStatus string

const (
	IdentityStatusVerified IdentityStatus = "verified"
	IdentityStatusPending  IdentityStatus = "pending"
	IdentityStatusFailed   IdentityStatus = "failed"
)

type IdentitySource string

const (
	IdentitySourceProvider IdentitySource = "provider"
	IdentitySourceManual   IdentitySource = "manual"
)

type AdapterIdentity struct {
	ID             uuid.UUID
	AdapterID      uuid.UUID
	Identity       string         // "user@example.com" or "example.com"
	IdentityType   IdentityType   // "email" or "domain"
	Status         IdentityStatus // "verified", "pending", "failed"
	SendingEnabled bool
	IsDefault      bool           // only email-type can be default
	DisplayName    *string        // optional friendly name for From header
	Source         IdentitySource // "provider" or "manual"
	LastSyncedAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

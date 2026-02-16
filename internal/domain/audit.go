package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditAction string

const (
	AuditCreate  AuditAction = "create"
	AuditUpdate  AuditAction = "update"
	AuditDelete  AuditAction = "delete"
	AuditPublish AuditAction = "publish"
	AuditDisable AuditAction = "disable"
	AuditEnable  AuditAction = "enable"
	AuditRevoke  AuditAction = "revoke"
	AuditPurge   AuditAction = "purge"
	AuditLogin   AuditAction = "login"
)

type AuditLog struct {
	ID          uuid.UUID
	ActorID     uuid.UUID
	ActorEmail  string
	Action      AuditAction
	EntityType  string // "tenant", "workspace", "template", etc.
	EntityID    uuid.UUID
	TenantID    *uuid.UUID
	WorkspaceID *uuid.UUID
	ScopeType   ScopeType
	Changes     map[string]any
	Metadata    map[string]any
	IPAddress   string
	CreatedAt   time.Time
}

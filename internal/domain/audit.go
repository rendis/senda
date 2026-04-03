package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditAction string

const (
	AuditCreate     AuditAction = "create"
	AuditUpdate     AuditAction = "update"
	AuditDelete     AuditAction = "delete"
	AuditPurge      AuditAction = "purge"
	AuditPublish    AuditAction = "publish"
	AuditArchive    AuditAction = "archive"
	AuditDisable    AuditAction = "disable"
	AuditEnable     AuditAction = "enable"
	AuditBulkSend   AuditAction = "bulk_send"
	AuditRevoke     AuditAction = "revoke"
	AuditInvite     AuditAction = "invite"
	AuditRemoveRole AuditAction = "remove_role"
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
	CreatedAt   time.Time
}

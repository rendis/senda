package handler

import (
	"sort"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

func appendAuditLog(c *echo.Context, store port.AuditLogStore, entry *domain.AuditLog) {
	if store == nil || entry == nil {
		return
	}

	member, _ := c.Get(middleware.ContextKeyMember).(*domain.Member)
	if member == nil {
		return
	}

	entry.ActorID = member.ID
	entry.ActorEmail = member.Email

	_ = store.Append(c.Request().Context(), entry)
}

func newWorkspaceAccessAuditChanges(before, after []service.WorkspaceAccessGrant) map[string]any {
	beforeIDs := grantedWorkspaceIDStrings(before)
	afterIDs := grantedWorkspaceIDStrings(after)

	beforeSet := make(map[string]struct{}, len(beforeIDs))
	for _, id := range beforeIDs {
		beforeSet[id] = struct{}{}
	}
	afterSet := make(map[string]struct{}, len(afterIDs))
	for _, id := range afterIDs {
		afterSet[id] = struct{}{}
	}

	granted := make([]string, 0)
	for _, id := range afterIDs {
		if _, ok := beforeSet[id]; !ok {
			granted = append(granted, id)
		}
	}

	revoked := make([]string, 0)
	for _, id := range beforeIDs {
		if _, ok := afterSet[id]; !ok {
			revoked = append(revoked, id)
		}
	}

	return map[string]any{
		"before_workspace_ids":  beforeIDs,
		"after_workspace_ids":   afterIDs,
		"granted_workspace_ids": granted,
		"revoked_workspace_ids": revoked,
	}
}

func grantedWorkspaceIDStrings(grants []service.WorkspaceAccessGrant) []string {
	ids := make([]string, 0, len(grants))
	for _, grant := range grants {
		if !grant.Granted {
			continue
		}
		ids = append(ids, grant.Workspace.ID.String())
	}
	sort.Strings(ids)
	return ids
}

func newAuditEntry(entityType string, entityID uuid.UUID, tenantID, workspaceID uuid.UUID, changes map[string]any, metadata map[string]any) *domain.AuditLog {
	return &domain.AuditLog{
		ID:          uuid.Must(uuid.NewV7()),
		Action:      domain.AuditUpdate,
		EntityType:  entityType,
		EntityID:    entityID,
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		ScopeType:   domain.ScopeWorkspace,
		Changes:     changes,
		Metadata:    metadata,
	}
}

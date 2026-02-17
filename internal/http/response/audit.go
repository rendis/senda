package response

import (
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// AuditLogResponse is the JSON response for an audit log entry.
type AuditLogResponse struct {
	ID          string         `json:"id"`
	ActorID     string         `json:"actor_id"`
	ActorEmail  string         `json:"actor_email"`
	Action      string         `json:"action"`
	EntityType  string         `json:"entity_type"`
	EntityID    string         `json:"entity_id"`
	TenantID    *string        `json:"tenant_id,omitempty"`
	WorkspaceID *string        `json:"workspace_id,omitempty"`
	ScopeType   string         `json:"scope_type"`
	Changes     map[string]any `json:"changes,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   string         `json:"created_at"`
}

// AuditLogListResponse is the JSON response for a paginated list of audit logs.
type AuditLogListResponse struct {
	Items      []AuditLogResponse `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
	HasMore    bool               `json:"has_more"`
}

// NewAuditLogResponse maps a domain AuditLog to an AuditLogResponse.
func NewAuditLogResponse(a *domain.AuditLog) AuditLogResponse {
	resp := AuditLogResponse{
		ID:         a.ID.String(),
		ActorID:    a.ActorID.String(),
		ActorEmail: a.ActorEmail,
		Action:     string(a.Action),
		EntityType: a.EntityType,
		EntityID:   a.EntityID.String(),
		ScopeType:  string(a.ScopeType),
		Changes:    a.Changes,
		Metadata:   a.Metadata,
		CreatedAt:  formatTime(a.CreatedAt),
	}
	if a.TenantID != nil {
		s := a.TenantID.String()
		resp.TenantID = &s
	}
	if a.WorkspaceID != nil {
		s := a.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	return resp
}

// NewAuditLogListResponse maps a PageResult of audit logs to an AuditLogListResponse.
func NewAuditLogListResponse(page *port.PageResult[domain.AuditLog]) AuditLogListResponse {
	items := make([]AuditLogResponse, len(page.Items))
	for i, a := range page.Items {
		items[i] = NewAuditLogResponse(a)
	}
	return AuditLogListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}

package response

import (
	"github.com/senda-app/senda/internal/domain"
)

// MemberResponse is the JSON response for a single member.
type MemberResponse struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// MemberRoleResponse is the JSON response for a single member role assignment.
type MemberRoleResponse struct {
	ID          string  `json:"id"`
	MemberID    string  `json:"member_id"`
	Role        string  `json:"role"`
	ScopeType   string  `json:"scope_type"`
	TenantID    *string `json:"tenant_id,omitempty"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// MemberWithRolesResponse returns a member with their role assignments.
type MemberWithRolesResponse struct {
	MemberResponse
	Roles []MemberRoleResponse `json:"roles"`
}

// MemberListResponse is the paginated list of members with roles.
type MemberListResponse struct {
	Items      []MemberWithRolesResponse `json:"items"`
	NextCursor string                    `json:"next_cursor,omitempty"`
	HasMore    bool                      `json:"has_more"`
}

// NewMemberResponse maps a domain Member to a MemberResponse.
func NewMemberResponse(m *domain.Member) MemberResponse {
	return MemberResponse{
		ID:          m.ID.String(),
		Email:       m.Email,
		DisplayName: m.DisplayName,
		CreatedAt:   m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   m.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// NewMemberRoleResponse maps a domain MemberRole to a MemberRoleResponse.
func NewMemberRoleResponse(r *domain.MemberRole) MemberRoleResponse {
	resp := MemberRoleResponse{
		ID:        r.ID.String(),
		MemberID:  r.MemberID.String(),
		Role:      string(r.Role),
		ScopeType: string(r.ScopeType),
		CreatedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if r.TenantID != nil {
		s := r.TenantID.String()
		resp.TenantID = &s
	}
	if r.WorkspaceID != nil {
		s := r.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	return resp
}

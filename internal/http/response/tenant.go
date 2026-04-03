package response

import (
	"github.com/rendis/senda/internal/domain"
)

// TenantResponse is the JSON response for a single tenant.
type TenantResponse struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	DeleteBlockedReason string `json:"delete_blocked_reason,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TenantListResponse is the JSON response for a paginated list of tenants.
type TenantListResponse struct {
	Items      []TenantResponse `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// NewTenantResponse maps a domain Tenant to a TenantResponse.
func NewTenantResponse(t *domain.Tenant) TenantResponse {
	return TenantResponse{
		ID:        t.ID.String(),
		Code:      t.Code,
		Name:      t.Name,
		IsActive:  t.IsActive,
		CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: t.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

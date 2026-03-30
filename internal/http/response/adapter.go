package response

import (
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// AdapterResponse is the JSON response for an adapter.
// SECURITY: ConfigEncrypted is never exposed. Only non-sensitive metadata is shown.
type AdapterResponse struct {
	ID                 string            `json:"id"`
	WorkspaceID        *string           `json:"workspace_id,omitempty"`
	Name               string            `json:"name"`
	AdapterType        string            `json:"adapter_type"`
	IsDefault          bool              `json:"is_default"`
	RateLimitPerSecond int               `json:"rate_limit_per_second"`
	ConfigMeta         map[string]string `json:"config_meta,omitempty"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
}

// AdapterListResponse is the JSON response for a paginated list of adapters.
type AdapterListResponse struct {
	Items      []AdapterResponse `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
	HasMore    bool              `json:"has_more"`
}

// NewAdapterResponse maps a domain Adapter to an AdapterResponse.
func NewAdapterResponse(a *domain.Adapter) AdapterResponse {
	resp := AdapterResponse{
		ID:                 a.ID.String(),
		Name:               a.Name,
		AdapterType:        string(a.AdapterType),
		IsDefault:          a.IsDefault,
		RateLimitPerSecond: a.RateLimitPerSecond,
		ConfigMeta:         a.ConfigMeta,
		CreatedAt:          a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:          a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if a.WorkspaceID != nil {
		s := a.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	return resp
}

// NewAdapterListResponse maps a PageResult of adapters to an AdapterListResponse.
func NewAdapterListResponse(page *port.PageResult[domain.Adapter]) AdapterListResponse {
	items := make([]AdapterResponse, len(page.Items))
	for i, a := range page.Items {
		items[i] = NewAdapterResponse(a)
	}
	return AdapterListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}

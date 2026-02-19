package response

import "github.com/senda-app/senda/internal/domain"

// AdapterIdentityResponse is the JSON response for an adapter identity.
type AdapterIdentityResponse struct {
	ID             string  `json:"id"`
	AdapterID      string  `json:"adapter_id"`
	Identity       string  `json:"identity"`
	IdentityType   string  `json:"identity_type"`
	Status         string  `json:"status"`
	SendingEnabled bool    `json:"sending_enabled"`
	IsDefault      bool    `json:"is_default"`
	DisplayName    *string `json:"display_name,omitempty"`
	Source         string  `json:"source"`
	LastSyncedAt   *string `json:"last_synced_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// NewAdapterIdentityResponse maps a domain AdapterIdentity to a response.
func NewAdapterIdentityResponse(ai *domain.AdapterIdentity) AdapterIdentityResponse {
	resp := AdapterIdentityResponse{
		ID:             ai.ID.String(),
		AdapterID:      ai.AdapterID.String(),
		Identity:       ai.Identity,
		IdentityType:   string(ai.IdentityType),
		Status:         string(ai.Status),
		SendingEnabled: ai.SendingEnabled,
		IsDefault:      ai.IsDefault,
		DisplayName:    ai.DisplayName,
		Source:         ai.Source,
		CreatedAt:      ai.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      ai.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if ai.LastSyncedAt != nil {
		s := ai.LastSyncedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.LastSyncedAt = &s
	}
	return resp
}

// NewAdapterIdentityListResponse maps a slice of identities to responses.
func NewAdapterIdentityListResponse(identities []*domain.AdapterIdentity) []AdapterIdentityResponse {
	items := make([]AdapterIdentityResponse, len(identities))
	for i, ai := range identities {
		items[i] = NewAdapterIdentityResponse(ai)
	}
	return items
}

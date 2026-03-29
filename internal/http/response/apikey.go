package response

import (
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// APIKeyCreatedResponse includes the full key (only returned once at creation).
// SECURITY: The full key is ONLY returned here, never in List/Get.
type APIKeyCreatedResponse struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Hint      string `json:"hint"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
}

// APIKeyResponse for List — NEVER includes key or hash.
type APIKeyResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Hint       string  `json:"hint"`
	CreatedAt  string  `json:"created_at"`
	CreatedBy  string  `json:"created_by"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	RevokedAt  *string `json:"revoked_at,omitempty"`
}

// APIKeyListResponse is the JSON response for a paginated list of API keys.
type APIKeyListResponse struct {
	Items      []APIKeyResponse `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// NewAPIKeyCreatedResponse maps a domain APIKey + full key to a creation response.
func NewAPIKeyCreatedResponse(key *domain.APIKey, fullKey string) APIKeyCreatedResponse {
	return APIKeyCreatedResponse{
		ID:        key.ID.String(),
		Key:       fullKey,
		Name:      key.Name,
		Hint:      key.KeyHint,
		CreatedAt: key.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy: key.CreatedBy.String(),
	}
}

// NewAPIKeyResponse maps a domain APIKey to an APIKeyResponse.
func NewAPIKeyResponse(key *domain.APIKey) APIKeyResponse {
	resp := APIKeyResponse{
		ID:        key.ID.String(),
		Name:      key.Name,
		Hint:      key.KeyHint,
		CreatedAt: key.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy: key.CreatedBy.String(),
	}
	if key.LastUsedAt != nil {
		s := key.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.LastUsedAt = &s
	}
	if key.RevokedAt != nil {
		s := key.RevokedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.RevokedAt = &s
	}
	return resp
}

// NewAPIKeyListResponse maps a PageResult of API keys to an APIKeyListResponse.
func NewAPIKeyListResponse(page *port.PageResult[domain.APIKey]) APIKeyListResponse {
	items := make([]APIKeyResponse, len(page.Items))
	for i, k := range page.Items {
		items[i] = NewAPIKeyResponse(k)
	}
	return APIKeyListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}

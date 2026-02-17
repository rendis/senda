package response

import (
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// WebhookCreatedResponse includes the secret (only returned once at creation).
// SECURITY: The secret is ONLY returned here, never in List/Get.
type WebhookCreatedResponse struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Secret    string   `json:"secret"`
	Events    []string `json:"events"`
	IsActive  bool     `json:"is_active"`
	CreatedAt string   `json:"created_at"`
}

// WebhookResponse for List/Get — NEVER includes secret.
type WebhookResponse struct {
	ID                  string   `json:"id"`
	URL                 string   `json:"url"`
	Events              []string `json:"events"`
	IsActive            bool     `json:"is_active"`
	ConsecutiveFailures int      `json:"consecutive_failures"`
	LastFailureAt       *string  `json:"last_failure_at,omitempty"`
	DisabledAt          *string  `json:"disabled_at,omitempty"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

// WebhookListResponse is the JSON response for a paginated list of webhooks.
type WebhookListResponse struct {
	Items      []WebhookResponse `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
	HasMore    bool              `json:"has_more"`
}

// NewWebhookCreatedResponse maps a domain Webhook to a creation response (includes secret).
func NewWebhookCreatedResponse(wh *domain.Webhook) WebhookCreatedResponse {
	return WebhookCreatedResponse{
		ID:        wh.ID.String(),
		URL:       wh.URL,
		Secret:    wh.Secret,
		Events:    wh.Events,
		IsActive:  wh.IsActive,
		CreatedAt: wh.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// NewWebhookResponse maps a domain Webhook to a WebhookResponse (secret hidden).
func NewWebhookResponse(wh *domain.Webhook) WebhookResponse {
	resp := WebhookResponse{
		ID:                  wh.ID.String(),
		URL:                 wh.URL,
		Events:              wh.Events,
		IsActive:            wh.IsActive,
		ConsecutiveFailures: wh.ConsecutiveFailures,
		CreatedAt:           wh.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           wh.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if wh.LastFailureAt != nil {
		s := wh.LastFailureAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.LastFailureAt = &s
	}
	if wh.DisabledAt != nil {
		s := wh.DisabledAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.DisabledAt = &s
	}
	return resp
}

// NewWebhookListResponse maps a PageResult of webhooks to a WebhookListResponse.
func NewWebhookListResponse(page *port.PageResult[domain.Webhook]) WebhookListResponse {
	items := make([]WebhookResponse, len(page.Items))
	for i, wh := range page.Items {
		items[i] = NewWebhookResponse(wh)
	}
	return WebhookListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}

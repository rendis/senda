package request

// CreateWebhookRequest is the request body for POST webhooks.
type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"` // e.g., ["email.sent", "email.delivered"] or ["*"]
}

// UpdateWebhookRequest is the request body for PUT webhooks/:id.
type UpdateWebhookRequest struct {
	URL      *string   `json:"url,omitempty"`
	Events   *[]string `json:"events,omitempty"`
	IsActive *bool     `json:"is_active,omitempty"`
}

package response

import "time"

// UnsubscribeContextResponse is returned by GET /api/v1/u/:token.
type UnsubscribeContextResponse struct {
	WorkspaceName    string `json:"workspace_name"`
	TemplateTypeSlug string `json:"template_type_slug"`
	TemplateTypeName string `json:"template_type_name"`
	Email            string `json:"email"`
	OptedOutOfType   bool   `json:"opted_out_of_type"`
	OptedOutOfAll    bool   `json:"opted_out_of_all"`
}

// PreferencesEntryResponse is one row in the preference center view.
type PreferencesEntryResponse struct {
	TemplateTypeSlug string    `json:"template_type_slug"`
	TemplateTypeName string    `json:"template_type_name"`
	Description      *string   `json:"description,omitempty"`
	Subscribed       bool      `json:"subscribed"`
	LastReceivedAt   time.Time `json:"last_received_at"`
}

// PreferencesViewResponse is returned by GET /api/v1/u/:token/preferences.
type PreferencesViewResponse struct {
	WorkspaceName string                     `json:"workspace_name"`
	Email         string                     `json:"email"`
	OptedOutOfAll bool                       `json:"opted_out_of_all"`
	Entries       []PreferencesEntryResponse `json:"entries"`
}

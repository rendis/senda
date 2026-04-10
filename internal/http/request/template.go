package request

import "encoding/json"

// --- Template Types ---

// CreateTemplateTypeRequest is the request body for POST template-types.
type CreateTemplateTypeRequest struct {
	Slug             string          `json:"slug"`
	Name             string          `json:"name"`
	Description      *string         `json:"description,omitempty"`
	AdapterID        *string         `json:"adapter_id,omitempty"`
	SenderIdentityID *string         `json:"sender_identity_id,omitempty"`
	VariableSchema   json.RawMessage `json:"variable_schema,omitempty"`
	TestRecipientMode      *string   `json:"test_recipient_mode,omitempty"`
	TestRecipientAddresses []string  `json:"test_recipient_addresses,omitempty"`
}

// UpdateTemplateTypeRequest is the request body for PUT template-types/:slug.
type UpdateTemplateTypeRequest struct {
	Slug             *string          `json:"slug,omitempty"`
	Name             *string          `json:"name,omitempty"`
	Description      *string          `json:"description,omitempty"`
	AdapterID        *string          `json:"adapter_id,omitempty"`
	SenderIdentityID *string          `json:"sender_identity_id,omitempty"`
	VariableSchema   *json.RawMessage `json:"variable_schema,omitempty"`
	TestRecipientMode      *string    `json:"test_recipient_mode,omitempty"`
	TestRecipientAddresses *[]string  `json:"test_recipient_addresses,omitempty"`
}

// --- Templates ---

// CreateTemplateRequest is the request body for POST templates.
type CreateTemplateRequest struct {
	TemplateTypeID string `json:"template_type_id"`
}

// --- Template Versions ---

// CreateVersionRequest is the request body for POST templates/:template_id/versions.
type CreateVersionRequest struct {
	Subject       string          `json:"subject"`
	PreviewText   string          `json:"preview_text"`
	FromName      string          `json:"from_name"`
	ReplyTo       *string         `json:"reply_to,omitempty"`
	BodyMJML      string          `json:"body_mjml"`
	DefaultLocale string          `json:"default_locale"`
	EditorData    json.RawMessage `json:"editor_data,omitempty"`
}

// --- Template Version Locales ---

// SetLocaleRequest is the request body for POST/PUT .../locales/:locale.
type SetLocaleRequest struct {
	Subject     *string         `json:"subject,omitempty"`
	PreviewText *string         `json:"preview_text,omitempty"`
	FromName    *string         `json:"from_name,omitempty"`
	BodyMJML    *string         `json:"body_mjml,omitempty"`
	EditorData  json.RawMessage `json:"editor_data,omitempty"`
}

// --- MJML Preview ---

// MJMLPreviewRequest is the request body for POST .../preview-mjml.
type MJMLPreviewRequest struct {
	MJML string `json:"mjml"`
}

// TemplateBulkSendRequest is the request body for POST .../templates/:template_id/bulk-send.
type TemplateBulkSendRequest struct {
	Items []SendBatchItemRequest `json:"items"`
}

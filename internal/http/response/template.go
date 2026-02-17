package response

import (
	"encoding/json"
	"time"

	"github.com/senda-app/senda/internal/domain"
)

// --- Template Type ---

// TemplateTypeResponse is the JSON response for a template type.
type TemplateTypeResponse struct {
	ID             string           `json:"id"`
	WorkspaceID    *string          `json:"workspace_id,omitempty"`
	Slug           string           `json:"slug"`
	Name           string           `json:"name"`
	Description    *string          `json:"description,omitempty"`
	AdapterID      *string          `json:"adapter_id,omitempty"`
	VariableSchema *json.RawMessage `json:"variable_schema,omitempty"`
	CreatedAt      string           `json:"created_at"`
	UpdatedAt      string           `json:"updated_at"`
}

// TemplateTypeListResponse is the JSON response for a list of template types.
type TemplateTypeListResponse struct {
	Items []TemplateTypeResponse `json:"items"`
}

// NewTemplateTypeResponse maps a domain TemplateType to a TemplateTypeResponse.
func NewTemplateTypeResponse(tt *domain.TemplateType) TemplateTypeResponse {
	resp := TemplateTypeResponse{
		ID:        tt.ID.String(),
		Slug:      tt.Slug,
		Name:      tt.Name,
		CreatedAt: formatTime(tt.CreatedAt),
		UpdatedAt: formatTime(tt.UpdatedAt),
	}
	if tt.WorkspaceID != nil {
		s := tt.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	if tt.Description != nil {
		resp.Description = tt.Description
	}
	if tt.AdapterID != nil {
		s := tt.AdapterID.String()
		resp.AdapterID = &s
	}
	if tt.VariableSchema != nil {
		raw, err := json.Marshal(tt.VariableSchema)
		if err == nil {
			rm := json.RawMessage(raw)
			resp.VariableSchema = &rm
		}
	}
	return resp
}

// NewTemplateTypeListResponse maps a slice of template types to a list response.
func NewTemplateTypeListResponse(types []*domain.TemplateType) TemplateTypeListResponse {
	items := make([]TemplateTypeResponse, len(types))
	for i, tt := range types {
		items[i] = NewTemplateTypeResponse(tt)
	}
	return TemplateTypeListResponse{Items: items}
}

// --- Template ---

// TemplateResponse is the JSON response for a template.
type TemplateResponse struct {
	ID             string  `json:"id"`
	TemplateTypeID string  `json:"template_type_id"`
	WorkspaceID    *string `json:"workspace_id,omitempty"`
	IsDisabled     bool    `json:"is_disabled"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// NewTemplateResponse maps a domain Template to a TemplateResponse.
func NewTemplateResponse(t *domain.Template) TemplateResponse {
	resp := TemplateResponse{
		ID:             t.ID.String(),
		TemplateTypeID: t.TemplateTypeID.String(),
		IsDisabled:     t.IsDisabled,
		CreatedAt:      formatTime(t.CreatedAt),
		UpdatedAt:      formatTime(t.UpdatedAt),
	}
	if t.WorkspaceID != nil {
		s := t.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	return resp
}

// --- Template Version ---

// TemplateVersionResponse is the JSON response for a template version.
type TemplateVersionResponse struct {
	ID            string           `json:"id"`
	TemplateID    string           `json:"template_id"`
	VersionNumber int              `json:"version_number"`
	Status        string           `json:"status"`
	Subject       string           `json:"subject"`
	PreviewText   string           `json:"preview_text"`
	FromName      string           `json:"from_name"`
	FromEmail     string           `json:"from_email"`
	ReplyTo       *string          `json:"reply_to,omitempty"`
	DefaultLocale string           `json:"default_locale"`
	EditorData    *json.RawMessage `json:"editor_data,omitempty"`
	CreatedBy     *string          `json:"created_by,omitempty"`
	PublishedAt   *string          `json:"published_at,omitempty"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
}

// TemplateVersionListResponse is the JSON response for a list of template versions.
type TemplateVersionListResponse struct {
	Items []TemplateVersionResponse `json:"items"`
}

// NewTemplateVersionResponse maps a domain TemplateVersion to a response.
func NewTemplateVersionResponse(v *domain.TemplateVersion) TemplateVersionResponse {
	resp := TemplateVersionResponse{
		ID:            v.ID.String(),
		TemplateID:    v.TemplateID.String(),
		VersionNumber: v.VersionNumber,
		Status:        string(v.Status),
		Subject:       v.Subject,
		PreviewText:   v.PreviewText,
		FromName:      v.FromName,
		FromEmail:     v.FromEmail,
		ReplyTo:       v.ReplyTo,
		DefaultLocale: v.DefaultLocale,
		CreatedAt:     formatTime(v.CreatedAt),
		UpdatedAt:     formatTime(v.UpdatedAt),
	}
	if v.EditorData != nil {
		raw, err := json.Marshal(v.EditorData)
		if err == nil {
			rm := json.RawMessage(raw)
			resp.EditorData = &rm
		}
	}
	if v.CreatedBy != nil {
		s := v.CreatedBy.String()
		resp.CreatedBy = &s
	}
	if v.PublishedAt != nil {
		s := formatTime(*v.PublishedAt)
		resp.PublishedAt = &s
	}
	return resp
}

// NewTemplateVersionListResponse maps a slice of versions to a list response.
func NewTemplateVersionListResponse(versions []*domain.TemplateVersion) TemplateVersionListResponse {
	items := make([]TemplateVersionResponse, len(versions))
	for i, v := range versions {
		items[i] = NewTemplateVersionResponse(v)
	}
	return TemplateVersionListResponse{Items: items}
}

// --- Template Version Locale ---

// TemplateVersionLocaleResponse is the JSON response for a locale override.
type TemplateVersionLocaleResponse struct {
	ID                string           `json:"id"`
	TemplateVersionID string           `json:"template_version_id"`
	Locale            string           `json:"locale"`
	Subject           *string          `json:"subject,omitempty"`
	PreviewText       *string          `json:"preview_text,omitempty"`
	FromName          *string          `json:"from_name,omitempty"`
	BodyMJML          *string          `json:"body_mjml,omitempty"`
	EditorData        *json.RawMessage `json:"editor_data,omitempty"`
	CreatedAt         string           `json:"created_at"`
	UpdatedAt         string           `json:"updated_at"`
}

// NewTemplateVersionLocaleResponse maps a domain TemplateVersionLocale to a response.
func NewTemplateVersionLocaleResponse(l *domain.TemplateVersionLocale) TemplateVersionLocaleResponse {
	resp := TemplateVersionLocaleResponse{
		ID:                l.ID.String(),
		TemplateVersionID: l.TemplateVersionID.String(),
		Locale:            l.Locale,
		Subject:           l.Subject,
		PreviewText:       l.PreviewText,
		FromName:          l.FromName,
		BodyMJML:          l.BodyMJML,
		CreatedAt:         formatTime(l.CreatedAt),
		UpdatedAt:         formatTime(l.UpdatedAt),
	}
	if l.EditorData != nil {
		raw, err := json.Marshal(l.EditorData)
		if err == nil {
			rm := json.RawMessage(raw)
			resp.EditorData = &rm
		}
	}
	return resp
}

// --- MJML Preview ---

// MJMLPreviewResponse is the JSON response for MJML preview.
type MJMLPreviewResponse struct {
	HTML string `json:"html"`
}

// --- Helpers ---

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z07:00")
}

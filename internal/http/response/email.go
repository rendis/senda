package response

import (
	"github.com/senda-app/senda/internal/domain"
)

// EmailResponse is the JSON response for an email record.
type EmailResponse struct {
	ID                string   `json:"id"`
	TrackingID        string   `json:"tracking_id"`
	ExternalID        *string  `json:"external_id,omitempty"`
	WorkspaceID       string   `json:"workspace_id"`
	TenantID          string   `json:"tenant_id"`
	TemplateTypeSlug  string   `json:"template_type_slug"`
	TemplateRef       string   `json:"template_ref"`
	RecipientEmail    string   `json:"recipient_email"`
	CC                []string `json:"cc,omitempty"`
	BCC               []string `json:"bcc,omitempty"`
	FromEmail         string   `json:"from_email"`
	FromName          string   `json:"from_name"`
	ReplyTo           *string  `json:"reply_to,omitempty"`
	SubjectRendered   string   `json:"subject_rendered"`
	Locale            *string  `json:"locale,omitempty"`
	Status            string   `json:"status"`
	ProviderMessageID *string  `json:"provider_message_id,omitempty"`
	RetryCount        int      `json:"retry_count"`
	MaxRetries        int      `json:"max_retries"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

// EmailListResponse is the JSON response for a paginated list of emails.
type EmailListResponse struct {
	Items      []EmailResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
	HasMore    bool            `json:"has_more"`
}

// EmailEventResponse is the JSON response for an email event.
type EmailEventResponse struct {
	ID         string         `json:"id"`
	EmailID    string         `json:"email_id"`
	EventType  string         `json:"event_type"`
	OccurredAt string         `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  string         `json:"created_at"`
}

// EmailDetailResponse is the JSON response for a single email with events.
type EmailDetailResponse struct {
	EmailResponse
	Events []EmailEventResponse `json:"events"`
}

// NewEmailResponse maps a domain Email to an EmailResponse.
func NewEmailResponse(e *domain.Email) EmailResponse {
	return EmailResponse{
		ID:                e.ID.String(),
		TrackingID:        e.TrackingID,
		ExternalID:        e.ExternalID,
		WorkspaceID:       e.WorkspaceID.String(),
		TenantID:          e.TenantID.String(),
		TemplateTypeSlug:  e.TemplateTypeSlug,
		TemplateRef:       e.TemplateRef,
		RecipientEmail:    e.RecipientEmail,
		CC:                e.CC,
		BCC:               e.BCC,
		FromEmail:         e.FromEmail,
		FromName:          e.FromName,
		ReplyTo:           e.ReplyTo,
		SubjectRendered:   e.SubjectRendered,
		Locale:            e.Locale,
		Status:            string(e.Status),
		ProviderMessageID: e.ProviderMessageID,
		RetryCount:        e.RetryCount,
		MaxRetries:        e.MaxRetries,
		CreatedAt:         formatTime(e.CreatedAt),
		UpdatedAt:         formatTime(e.UpdatedAt),
	}
}

// NewEmailListResponse maps a slice of emails with cursor to an EmailListResponse.
func NewEmailListResponse(emails []*domain.Email, nextCursor string) EmailListResponse {
	items := make([]EmailResponse, len(emails))
	for i, e := range emails {
		items[i] = NewEmailResponse(e)
	}
	return EmailListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}
}

// NewEmailEventResponse maps a domain EmailEvent to an EmailEventResponse.
func NewEmailEventResponse(ev *domain.EmailEvent) EmailEventResponse {
	return EmailEventResponse{
		ID:         ev.ID.String(),
		EmailID:    ev.EmailID.String(),
		EventType:  string(ev.EventType),
		OccurredAt: formatTime(ev.OccurredAt),
		Metadata:   ev.Metadata,
		CreatedAt:  formatTime(ev.CreatedAt),
	}
}

// NewEmailDetailResponse maps a domain Email and events to an EmailDetailResponse.
func NewEmailDetailResponse(e *domain.Email, events []*domain.EmailEvent) EmailDetailResponse {
	evts := make([]EmailEventResponse, len(events))
	for i, ev := range events {
		evts[i] = NewEmailEventResponse(ev)
	}
	return EmailDetailResponse{
		EmailResponse: NewEmailResponse(e),
		Events:        evts,
	}
}

// NewEmailEventListResponse maps a slice of events to a response.
func NewEmailEventListResponse(events []*domain.EmailEvent) []EmailEventResponse {
	items := make([]EmailEventResponse, len(events))
	for i, ev := range events {
		items[i] = NewEmailEventResponse(ev)
	}
	return items
}

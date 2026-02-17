package domain

import (
	"time"

	"github.com/google/uuid"
)

type EmailStatus string

const (
	StatusQueued     EmailStatus = "queued"
	StatusProcessing EmailStatus = "processing"
	StatusSent       EmailStatus = "sent"
	StatusDelivered  EmailStatus = "delivered"
	StatusOpened     EmailStatus = "opened"
	StatusBounced    EmailStatus = "bounced"
	StatusComplained EmailStatus = "complained"
	StatusFailed     EmailStatus = "failed"
	StatusSuppressed EmailStatus = "suppressed"
)

type Email struct {
	ID                uuid.UUID
	TrackingID        string
	ExternalID        *string
	WorkspaceID       uuid.UUID
	TenantID          uuid.UUID
	TemplateID        uuid.UUID
	TemplateVersionID uuid.UUID
	TemplateTypeSlug  string
	TemplateRef       string // original "latam:acme:welcome"
	RecipientEmail    string
	CC                []string
	BCC               []string
	FromEmail         string
	FromName          string
	ReplyTo           *string
	SubjectRendered   string
	Locale            *string
	Status            EmailStatus
	AdapterID         uuid.UUID
	ProviderMessageID *string
	VariablesSnapshot map[string]any
	InjectorsSnapshot map[string]map[string]any
	BodyMJML          string // MJML source snapshot (rendered with variables before compile)
	RetryCount        int
	MaxRetries        int
	NextRetryAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type EmailEvent struct {
	ID         uuid.UUID
	EmailID    uuid.UUID
	EventType  EmailStatus
	OccurredAt time.Time
	Metadata   map[string]any
	CreatedAt  time.Time
}

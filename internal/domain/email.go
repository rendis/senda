package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrStatusConflict = errors.New("status transition conflict")

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

// EmailEventType represents the type of event that occurred on an email.
type EmailEventType string

const (
	EventTypeQueued     EmailEventType = "queued"
	EventTypeProcessing EmailEventType = "processing"
	EventTypeSent       EmailEventType = "sent"
	EventTypeDelivered  EmailEventType = "delivered"
	EventTypeBounced    EmailEventType = "bounced"
	EventTypeComplained EmailEventType = "complained"
	EventTypeOpened     EmailEventType = "opened"
	EventTypeFailed     EmailEventType = "failed"
	EventTypeSuppressed EmailEventType = "suppressed"
)

// StatusToEventType maps an EmailStatus to its corresponding EmailEventType.
func StatusToEventType(s EmailStatus) EmailEventType {
	return EmailEventType(s)
}

type EmailSourceType string

const (
	EmailSourceTypeDataPlaneAPIKey              EmailSourceType = "data_plane_api_key"
	EmailSourceTypeManagementTemplateBulkUpload EmailSourceType = "management_template_bulk_upload"
)

type Email struct {
	ID                  uuid.UUID
	TrackingID          string
	ExternalID          *string
	WorkspaceID         uuid.UUID
	TenantID            uuid.UUID
	TemplateID          uuid.UUID
	TemplateVersionID   uuid.UUID
	TemplateTypeSlug    string
	TemplateRef         string // original "latam:acme:welcome"
	RecipientEmail      string
	CC                  []string
	BCC                 []string
	FromEmail           string
	FromName            string
	ReplyTo             *string
	SubjectRendered     string
	Locale              *string
	Status              EmailStatus
	AdapterID           uuid.UUID
	ProviderMessageID   *string
	VariablesSnapshot   map[string]any
	InjectorsSnapshot   map[string]map[string]any
	SourceType          EmailSourceType
	SourceActorMemberID *uuid.UUID
	SourceActorEmail    *string
	BodyMJML            string // MJML source snapshot (rendered with variables before compile)
	RetryCount          int
	OpenTrackingEnabled bool
	MaxRetries          int
	NextRetryAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type EmailEvent struct {
	ID         uuid.UUID
	EmailID    uuid.UUID
	EventType  EmailEventType
	OccurredAt time.Time
	Metadata   map[string]any
	CreatedAt  time.Time
}

package domain

import (
	"time"

	"github.com/google/uuid"
)

type VersionStatus string

const (
	VersionStatusDraft     VersionStatus = "draft"
	VersionStatusPublished VersionStatus = "published"
	VersionStatusArchived  VersionStatus = "archived"
)

type TemplateType struct {
	ID             uuid.UUID
	WorkspaceID    *uuid.UUID // nil = global
	Slug           string
	Name           string
	Description    *string
	AdapterID      *uuid.UUID     // adapter assigned by admin; nil = not configured yet
	VariableSchema map[string]any // JSON Schema for event variables
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type Template struct {
	ID             uuid.UUID
	WorkspaceID    *uuid.UUID // nil = global
	TemplateTypeID uuid.UUID
	Slug           string
	Name           string
	IsDisabled     bool // kill switch
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type TemplateVersion struct {
	ID            uuid.UUID
	TemplateID    uuid.UUID
	VersionNumber int
	Status        VersionStatus
	Subject       string
	PreviewText   *string
	FromEmail     string
	FromName      string
	ReplyTo       *string
	BodyMJML      string // MJML source
	EditorData    map[string]any // JSONB editor state
	DefaultLocale string
	CreatedBy     *uuid.UUID // member who created the version
	PublishedAt   *time.Time
	ArchivedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type TemplateVersionLocale struct {
	ID          uuid.UUID
	VersionID   uuid.UUID
	Locale      string // e.g., "es", "pt-BR"
	Subject     *string
	PreviewText *string
	FromName    *string
	BodyMJML    *string // nil = use default body
	EditorData  map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

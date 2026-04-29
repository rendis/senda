package domain

import (
	"time"

	"github.com/google/uuid"
)

type AdapterType string

const (
	AdapterTypeSES   AdapterType = "ses"
	AdapterTypeGmail AdapterType = "gmail"
	AdapterTypeSMTP  AdapterType = "smtp"
)

type Adapter struct {
	ID                 uuid.UUID
	WorkspaceID        *uuid.UUID        // nil = global
	Name               string
	AdapterType        AdapterType
	ConfigEncrypted    []byte            // AES-256-GCM encrypted JSON
	IsDefault          bool
	RateLimitPerSecond int
	ConfigMeta         map[string]string // non-sensitive config fields (region, delegate_email)
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}

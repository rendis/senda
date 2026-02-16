package domain

import (
	"time"

	"github.com/google/uuid"
)

type AdapterType string

const (
	AdapterTypeSES   AdapterType = "ses"
	AdapterTypeGmail AdapterType = "gmail"
)

type Adapter struct {
	ID                  uuid.UUID
	WorkspaceID         *uuid.UUID // nil = global
	Name                string
	AdapterType         AdapterType
	ConfigEncrypted     []byte // AES-256-GCM encrypted JSON
	IsDefault           bool
	RateLimitPerSecond  int
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
}

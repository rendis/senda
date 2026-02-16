package domain

import (
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	KeyHash     string // SHA-256 of the raw key
	KeyPrefix   string // first 8 chars for identification
	KeyHint     string // last 8 chars for UI display
	CreatedBy   uuid.UUID // member who created the key
	LastUsedAt  *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

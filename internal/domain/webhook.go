package domain

import (
	"time"

	"github.com/google/uuid"
)

type Webhook struct {
	ID                  uuid.UUID
	WorkspaceID         uuid.UUID
	URL                 string
	Secret              string // HMAC signing secret
	Events              []string
	IsActive            bool
	ConsecutiveFailures int
	LastFailureAt       *time.Time
	DisabledAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// SubscribesTo checks if this webhook listens for the given event type.
func (w *Webhook) SubscribesTo(eventType string) bool {
	for _, e := range w.Events {
		if e == eventType || e == "*" {
			return true
		}
	}
	return false
}

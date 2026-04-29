package domain

import (
	"time"

	"github.com/google/uuid"
)

type SubscriptionSource string

const (
	SubscriptionSourceRecipientOptout SubscriptionSource = "recipient_optout"
	SubscriptionSourceRecipientOptin  SubscriptionSource = "recipient_optin"
	SubscriptionSourceAdmin           SubscriptionSource = "admin"
)

type TemplateTypeSubscription struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	TemplateTypeID uuid.UUID
	Email          string
	Subscribed     bool
	Source         SubscriptionSource
	SourceEmailID  *uuid.UUID
	ActorID        *uuid.UUID
	Notes          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

package domain

import (
	"encoding/json"
	"time"
)

// ProviderEventType represents the type of a normalized provider event.
type ProviderEventType string

const (
	EventDelivered  ProviderEventType = "delivered"
	EventBounced    ProviderEventType = "bounced"
	EventComplained ProviderEventType = "complained"
	EventOpened     ProviderEventType = "opened"
)

// BounceDetail contains bounce-specific information from the provider payload.
type BounceDetail struct {
	BounceType     string // "hard" or "soft"
	DiagnosticCode string
	Recipients     []string
}

// ComplaintDetail contains complaint-specific information from the provider payload.
type ComplaintDetail struct {
	ComplaintType string
	FeedbackID    string
	Recipients    []string
}

// ProviderEvent is the normalized event consumed by the EventProcessor.
type ProviderEvent struct {
	Type              ProviderEventType
	ProviderMessageID string
	Timestamp         time.Time
	RawPayload        json.RawMessage
	BounceDetail      *BounceDetail
	ComplaintDetail   *ComplaintDetail
}

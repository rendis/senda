package domain

import (
	"time"

	"github.com/google/uuid"
)

type BounceType string

const (
	BounceHard BounceType = "hard"
	BounceSoft BounceType = "soft"
)

type SuppressionReason string

const (
	SuppressionBounce    SuppressionReason = "bounce"
	SuppressionComplaint SuppressionReason = "complaint"
	SuppressionManual    SuppressionReason = "manual"
)

type SuppressionGlobal struct {
	ID            uuid.UUID
	Email         string
	Reason        SuppressionReason
	Source        string // "ses_webhook", "manual", etc.
	SourceEmailID *uuid.UUID
	Notes         *string
	CreatedAt     time.Time
	RemovedAt     *time.Time
	RemovedBy     *uuid.UUID
	RemovalReason *string
}

type SuppressionWorkspace struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	Email         string
	Reason        SuppressionReason
	Source        string
	SourceEmailID *uuid.UUID
	Notes         *string
	CreatedAt     time.Time
	RemovedAt     *time.Time
	RemovedBy     *uuid.UUID
	RemovalReason *string
}

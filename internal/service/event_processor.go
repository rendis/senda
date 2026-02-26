package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
)

// EmailLookup is a local interface for finding emails by provider message ID.
// The main EmailStore in port/ does not have GetByProviderMessageID yet.
type EmailLookup interface {
	GetByProviderMessageID(ctx context.Context, providerMessageID string) (*domain.Email, error)
}

// EmailStatusUpdater is a local interface for updating email status and adding events.
type EmailStatusUpdater interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.EmailStatus) error
	AddEvent(ctx context.Context, event *domain.EmailEvent) error
}

// SuppressionWriter is a local interface for adding suppression entries.
type SuppressionWriter interface {
	AddGlobal(ctx context.Context, entry *domain.SuppressionGlobal) error
	AddWorkspace(ctx context.Context, entry *domain.SuppressionWorkspace) error
}

// WebhookDispatcher is a local interface for dispatching events to workspace webhooks.
// The real implementation comes from WebhookService (HT-24).
type WebhookDispatcher interface {
	Dispatch(ctx context.Context, workspaceID uuid.UUID, eventType string, payload any) error
}

// EventProcessor processes provider events (bounces, complaints, deliveries, opens).
type EventProcessor struct {
	emailLookup    EmailLookup
	emailUpdater   EmailStatusUpdater
	suppression    SuppressionWriter
	webhookService WebhookDispatcher
	logger         *slog.Logger
}

// NewEventProcessor creates a new EventProcessor with the given dependencies.
func NewEventProcessor(
	emailLookup EmailLookup,
	emailUpdater EmailStatusUpdater,
	suppression SuppressionWriter,
	webhookService WebhookDispatcher,
	logger *slog.Logger,
) *EventProcessor {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventProcessor{
		emailLookup:    emailLookup,
		emailUpdater:   emailUpdater,
		suppression:    suppression,
		webhookService: webhookService,
		logger:         logger,
	}
}

// Process handles a single provider event.
func (p *EventProcessor) Process(ctx context.Context, event *domain.ProviderEvent) error {
	// 1. Look up the email by provider message ID.
	email, err := p.emailLookup.GetByProviderMessageID(ctx, event.ProviderMessageID)
	if err != nil {
		p.logger.WarnContext(ctx, "email not found for provider event",
			"provider_message_id", event.ProviderMessageID,
			"event_type", event.Type,
			"error", err,
		)
		return err
	}

	// 2. Map event type to internal status.
	status, ok := mapEventToStatus(event.Type)
	if !ok {
		p.logger.WarnContext(ctx, "unknown provider event type",
			"event_type", event.Type,
			"provider_message_id", event.ProviderMessageID,
		)
		return nil
	}

	// 3. Update email status.
	if err := p.emailUpdater.UpdateStatus(ctx, email.ID, status); err != nil {
		return err
	}

	// 4. Add email event.
	metadata := buildEventMetadata(event)
	emailEvent := &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  status,
		OccurredAt: event.Timestamp,
		Metadata:   metadata,
		CreatedAt:  time.Now().UTC(),
	}
	if err := p.emailUpdater.AddEvent(ctx, emailEvent); err != nil {
		return err
	}

	// 5. Suppression side-effects.
	if err := p.handleSuppression(ctx, event, email); err != nil {
		p.logger.ErrorContext(ctx, "failed to add suppression entry",
			"email_id", email.ID,
			"event_type", event.Type,
			"error", err,
		)
		// Don't return error — suppression failure should not block event processing.
	}

	// 6. Dispatch to workspace webhooks.
	if p.webhookService != nil {
		webhookPayload := map[string]any{
			"email_id":            email.ID.String(),
			"tracking_id":         email.TrackingID,
			"recipient":           email.RecipientEmail,
			"status":              string(status),
			"provider_message_id": event.ProviderMessageID,
			"timestamp":           event.Timestamp.Format(time.RFC3339),
		}
		if err := p.webhookService.Dispatch(ctx, email.WorkspaceID, "email."+string(event.Type), webhookPayload); err != nil {
			p.logger.ErrorContext(ctx, "failed to dispatch webhook",
				"email_id", email.ID,
				"event_type", event.Type,
				"error", err,
			)
			// Don't return error — webhook failure should not block event processing.
		}
	}

	return nil
}

// ProcessDirect handles an event for an email that has already been looked up.
// Used by the open-tracking pixel where we already have the email from tracking_id.
// Status is only updated if the email is in an eligible state (sent/delivered).
func (p *EventProcessor) ProcessDirect(ctx context.Context, email *domain.Email, event *domain.ProviderEvent) error {
	status, ok := mapEventToStatus(event.Type)
	if !ok {
		p.logger.WarnContext(ctx, "unknown provider event type",
			"event_type", event.Type,
			"email_id", email.ID,
		)
		return nil
	}

	// Only transition status for opens if email is sent or delivered.
	// Don't overwrite bounced/complained/failed, and don't re-set opened.
	if event.Type == domain.EventOpened {
		if email.Status == domain.StatusSent || email.Status == domain.StatusDelivered {
			if err := p.emailUpdater.UpdateStatus(ctx, email.ID, status); err != nil {
				return err
			}
		}
	} else {
		if err := p.emailUpdater.UpdateStatus(ctx, email.ID, status); err != nil {
			return err
		}
	}

	// Always record the event (every open gets logged).
	metadata := buildEventMetadata(event)
	metadata["source"] = "open_tracking_pixel"
	emailEvent := &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  status,
		OccurredAt: event.Timestamp,
		Metadata:   metadata,
		CreatedAt:  time.Now().UTC(),
	}
	if err := p.emailUpdater.AddEvent(ctx, emailEvent); err != nil {
		return err
	}

	// Dispatch to workspace webhooks.
	if p.webhookService != nil {
		webhookPayload := map[string]any{
			"email_id":    email.ID.String(),
			"tracking_id": email.TrackingID,
			"recipient":   email.RecipientEmail,
			"status":      string(status),
			"timestamp":   event.Timestamp.Format(time.RFC3339),
		}
		if err := p.webhookService.Dispatch(ctx, email.WorkspaceID, "email."+string(event.Type), webhookPayload); err != nil {
			p.logger.ErrorContext(ctx, "failed to dispatch webhook",
				"email_id", email.ID,
				"event_type", event.Type,
				"error", err,
			)
		}
	}

	return nil
}

// handleSuppression adds suppression entries for hard bounces and complaints.
func (p *EventProcessor) handleSuppression(ctx context.Context, event *domain.ProviderEvent, email *domain.Email) error {
	switch event.Type {
	case domain.EventBounced:
		if event.BounceDetail == nil || event.BounceDetail.BounceType != "hard" {
			return nil
		}
		return p.suppression.AddGlobal(ctx, &domain.SuppressionGlobal{
			ID:            uuid.Must(uuid.NewV7()),
			Email:         email.RecipientEmail,
			Reason:        domain.SuppressionHardBounce,
			SourceEmailID: &email.ID,
			CreatedAt:     time.Now().UTC(),
		})

	case domain.EventComplained:
		return p.suppression.AddWorkspace(ctx, &domain.SuppressionWorkspace{
			ID:            uuid.Must(uuid.NewV7()),
			WorkspaceID:   email.WorkspaceID,
			Email:         email.RecipientEmail,
			Reason:        domain.SuppressionComplaint,
			SourceEmailID: &email.ID,
			CreatedAt:     time.Now().UTC(),
		})

	default:
		return nil
	}
}

// mapEventToStatus maps a provider event type to an internal email status.
func mapEventToStatus(eventType domain.ProviderEventType) (domain.EmailStatus, bool) {
	switch eventType {
	case domain.EventDelivered:
		return domain.StatusDelivered, true
	case domain.EventBounced:
		return domain.StatusBounced, true
	case domain.EventComplained:
		return domain.StatusComplained, true
	case domain.EventOpened:
		return domain.StatusOpened, true
	default:
		return "", false
	}
}

// buildEventMetadata creates metadata for an email event based on the provider event.
func buildEventMetadata(event *domain.ProviderEvent) map[string]any {
	meta := map[string]any{
		"provider_message_id": event.ProviderMessageID,
		"source":              "provider_webhook",
	}
	if event.BounceDetail != nil {
		meta["bounce_type"] = event.BounceDetail.BounceType
		meta["diagnostic_code"] = event.BounceDetail.DiagnosticCode
		meta["recipients"] = event.BounceDetail.Recipients
	}
	if event.ComplaintDetail != nil {
		meta["complaint_type"] = event.ComplaintDetail.ComplaintType
		meta["feedback_id"] = event.ComplaintDetail.FeedbackID
		meta["recipients"] = event.ComplaintDetail.Recipients
	}
	return meta
}

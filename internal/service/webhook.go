package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/port"
)

// WebhookService handles dispatching events to registered webhooks.
type WebhookService struct {
	webhookStore port.WebhookStore
	queue        port.JobQueue
	logger       *slog.Logger
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(ws port.WebhookStore, q port.JobQueue, opts ...WebhookServiceOption) *WebhookService {
	svc := &WebhookService{webhookStore: ws, queue: q, logger: slog.Default()}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// WebhookServiceOption configures optional WebhookService dependencies.
type WebhookServiceOption func(*WebhookService)

// WithWebhookLogger sets a custom logger for the WebhookService.
func WithWebhookLogger(l *slog.Logger) WebhookServiceOption {
	return func(s *WebhookService) {
		if l != nil {
			s.logger = l
		}
	}
}

// Dispatch sends an event to all active webhooks in the workspace that subscribe to this event type.
// Each matching webhook gets a separate enqueued job for delivery.
// Dispatch is fire-and-forget: errors are logged but never propagated to the caller.
// This ensures webhook delivery failures do not block the main processing flow.
func (s *WebhookService) Dispatch(ctx context.Context, workspaceID uuid.UUID, eventType string, payload any) error {
	webhooks, err := s.webhookStore.GetActiveByWorkspace(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "webhook dispatch: failed to fetch active webhooks",
			"workspace_id", workspaceID,
			"event_type", eventType,
			"error", err,
		)
		return nil
	}
	if len(webhooks) == 0 {
		return nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		s.logger.ErrorContext(ctx, "webhook dispatch: failed to marshal payload",
			"workspace_id", workspaceID,
			"event_type", eventType,
			"error", err,
		)
		return nil
	}

	for _, wh := range webhooks {
		if !wh.SubscribesTo(eventType) {
			continue
		}
		if err := s.queue.EnqueueWebhook(ctx, &port.WebhookJob{
			WebhookID: wh.ID,
			EventType: eventType,
			Payload:   payloadBytes,
		}); err != nil {
			s.logger.ErrorContext(ctx, "webhook dispatch: failed to enqueue webhook job",
				"webhook_id", wh.ID,
				"workspace_id", workspaceID,
				"event_type", eventType,
				"error", err,
			)
			// Continue to next webhook — don't fail the entire dispatch.
			continue
		}
	}

	return nil
}

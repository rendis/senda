package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/service"
)

// snsSubscribeURLHostRe validates that SubscribeURL hosts are SNS endpoints.
var snsSubscribeURLHostRe = regexp.MustCompile(`^sns\.[a-z]{2}(-[a-z]+-\d+)\.amazonaws\.com$`)

// SNSVerifier verifies the authenticity of SNS messages.
type SNSVerifier interface {
	Verify(message []byte) error
}

// SubscriptionConfirmer fetches a URL to confirm an SNS subscription.
// Abstracted for testability.
type SubscriptionConfirmer interface {
	ConfirmSubscription(ctx context.Context, subscribeURL string) error
}

// SESWebhookHandler handles incoming SES event notifications via SNS.
// Route: POST /api/v1/webhooks/ses/inbound (NO AUTH — uses SNS signature verification).
type SESWebhookHandler struct {
	processor *service.EventProcessor
	verifier  SNSVerifier
	confirmer SubscriptionConfirmer
	logger    *slog.Logger
}

// NewSESWebhookHandler creates a new SES webhook handler.
func NewSESWebhookHandler(
	processor *service.EventProcessor,
	verifier SNSVerifier,
	confirmer SubscriptionConfirmer,
	logger *slog.Logger,
) *SESWebhookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SESWebhookHandler{
		processor: processor,
		verifier:  verifier,
		confirmer: confirmer,
		logger:    logger,
	}
}

// snsMessage represents the SNS message envelope.
type snsMessage struct {
	Type             string `json:"Type"`
	MessageId        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL,omitempty"`
}

// sesNotification represents the SES event inside the SNS Message field.
type sesNotification struct {
	NotificationType string       `json:"notificationType"`
	Mail             sesMail      `json:"mail"`
	Bounce           *sesBounce   `json:"bounce,omitempty"`
	Complaint        *sesComplaint `json:"complaint,omitempty"`
	Delivery         *sesDelivery  `json:"delivery,omitempty"`
}

type sesMail struct {
	MessageId string `json:"messageId"`
}

type sesBounce struct {
	BounceType        string `json:"bounceType"`
	BouncedRecipients []struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"bouncedRecipients"`
	Timestamp string `json:"timestamp"`
}

type sesComplaint struct {
	ComplaintFeedbackType string `json:"complaintFeedbackType"`
	ComplainedRecipients  []struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"complainedRecipients"`
	FeedbackId string `json:"feedbackId,omitempty"`
	Timestamp  string `json:"timestamp"`
}

type sesDelivery struct {
	Timestamp string `json:"timestamp"`
}

const maxBodySize = 256 * 1024 // 256 KB

// HandleInbound processes incoming SNS messages for SES events.
func (h *SESWebhookHandler) HandleInbound(c *echo.Context) error {
	ctx := c.Request().Context()

	// 1. Read full body (limited to 256KB).
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxBodySize))
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to read SNS message body", "error", err)
		return c.NoContent(http.StatusBadRequest)
	}

	// 2. Verify SNS signature.
	if h.verifier != nil {
		if err := h.verifier.Verify(body); err != nil {
			h.logger.WarnContext(ctx, "SNS signature verification failed", "error", err)
			return c.NoContent(http.StatusForbidden)
		}
	}

	// 3. Parse SNS envelope.
	var msg snsMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		h.logger.ErrorContext(ctx, "failed to parse SNS message", "error", err)
		return c.NoContent(http.StatusBadRequest)
	}

	// 3.5. Validate TopicArn format.
	if !strings.HasPrefix(msg.TopicArn, "arn:aws:sns:") {
		h.logger.WarnContext(ctx, "SNS message with invalid TopicArn",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageId,
		)
		return c.NoContent(http.StatusBadRequest)
	}

	// 4. Handle message type.
	switch msg.Type {
	case "SubscriptionConfirmation":
		return h.handleSubscriptionConfirmation(ctx, c, &msg)

	case "Notification":
		return h.handleNotification(ctx, c, &msg, body)

	default:
		h.logger.WarnContext(ctx, "unknown SNS message type", "type", msg.Type)
		// Return 200 to prevent SNS retries.
		return c.NoContent(http.StatusOK)
	}
}

// handleSubscriptionConfirmation auto-confirms the SNS subscription.
func (h *SESWebhookHandler) handleSubscriptionConfirmation(ctx context.Context, c *echo.Context, msg *snsMessage) error {
	if msg.SubscribeURL == "" {
		h.logger.WarnContext(ctx, "subscription confirmation missing SubscribeURL")
		return c.NoContent(http.StatusBadRequest)
	}

	// SSRF protection: validate SubscribeURL before fetching.
	if err := validateSubscribeURL(msg.SubscribeURL); err != nil {
		h.logger.WarnContext(ctx, "subscription confirmation URL validation failed",
			"subscribe_url", msg.SubscribeURL,
			"error", err,
		)
		return c.NoContent(http.StatusBadRequest)
	}

	h.logger.InfoContext(ctx, "confirming SNS subscription",
		"topic_arn", msg.TopicArn,
		"subscribe_url", msg.SubscribeURL,
	)

	if h.confirmer != nil {
		if err := h.confirmer.ConfirmSubscription(ctx, msg.SubscribeURL); err != nil {
			h.logger.ErrorContext(ctx, "failed to confirm SNS subscription",
				"topic_arn", msg.TopicArn,
				"error", err,
			)
			// Return 200 anyway — SNS will retry.
			return c.NoContent(http.StatusOK)
		}
	}

	return c.NoContent(http.StatusOK)
}

// validateSubscribeURL validates that the SubscribeURL points to an actual SNS endpoint.
// This prevents SSRF attacks where an attacker crafts a malicious SubscribeURL.
func validateSubscribeURL(subscribeURL string) error {
	parsed, err := url.Parse(subscribeURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be https, got %q", parsed.Scheme)
	}

	if !snsSubscribeURLHostRe.MatchString(parsed.Host) {
		return fmt.Errorf("host %q is not a valid SNS endpoint", parsed.Host)
	}

	return nil
}

// handleNotification processes an SES event notification.
func (h *SESWebhookHandler) handleNotification(ctx context.Context, c *echo.Context, msg *snsMessage, rawBody []byte) error {
	// Parse SES notification from the Message field.
	var notification sesNotification
	if err := json.Unmarshal([]byte(msg.Message), &notification); err != nil {
		h.logger.ErrorContext(ctx, "failed to parse SES notification",
			"message_id", msg.MessageId,
			"error", err,
		)
		// Return 200 to prevent SNS retries on malformed messages.
		return c.NoContent(http.StatusOK)
	}

	// Map SES notification to ProviderEvent.
	event := h.mapSESToProviderEvent(&notification, rawBody)
	if event == nil {
		h.logger.WarnContext(ctx, "unhandled SES notification type",
			"notification_type", notification.NotificationType,
		)
		return c.NoContent(http.StatusOK)
	}

	// Process the event (updates status, suppression, webhooks).
	if err := h.processor.Process(ctx, event); err != nil {
		h.logger.ErrorContext(ctx, "failed to process SES event",
			"notification_type", notification.NotificationType,
			"provider_message_id", event.ProviderMessageID,
			"error", err,
		)
		// Return 200 to prevent SNS retries — we log the error.
	}

	return c.NoContent(http.StatusOK)
}

// mapSESToProviderEvent converts an SES notification to a ProviderEvent.
func (h *SESWebhookHandler) mapSESToProviderEvent(n *sesNotification, rawBody []byte) *service.ProviderEvent {
	event := &service.ProviderEvent{
		ProviderMessageID: n.Mail.MessageId,
		RawPayload:        rawBody,
	}

	switch n.NotificationType {
	case "Delivery":
		event.Type = service.EventDelivered
		if n.Delivery != nil {
			event.Timestamp = parseSESTimestamp(n.Delivery.Timestamp)
		}

	case "Bounce":
		event.Type = service.EventBounced
		if n.Bounce != nil {
			event.Timestamp = parseSESTimestamp(n.Bounce.Timestamp)

			bounceType := "soft"
			if n.Bounce.BounceType == "Permanent" {
				bounceType = "hard"
			}

			recipients := make([]string, 0, len(n.Bounce.BouncedRecipients))
			for _, r := range n.Bounce.BouncedRecipients {
				recipients = append(recipients, r.EmailAddress)
			}

			event.BounceDetail = &service.BounceDetail{
				BounceType: bounceType,
				Recipients: recipients,
			}
		}

	case "Complaint":
		event.Type = service.EventComplained
		if n.Complaint != nil {
			event.Timestamp = parseSESTimestamp(n.Complaint.Timestamp)

			recipients := make([]string, 0, len(n.Complaint.ComplainedRecipients))
			for _, r := range n.Complaint.ComplainedRecipients {
				recipients = append(recipients, r.EmailAddress)
			}

			event.ComplaintDetail = &service.ComplaintDetail{
				ComplaintType: n.Complaint.ComplaintFeedbackType,
				FeedbackID:    n.Complaint.FeedbackId,
				Recipients:    recipients,
			}
		}

	default:
		return nil
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	return event
}

// parseSESTimestamp parses an SES timestamp string (ISO 8601).
func parseSESTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// SES sometimes uses a different format
		t, err = time.Parse("2006-01-02T15:04:05.000Z", ts)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

// HTTPSubscriptionConfirmer confirms SNS subscriptions by making an HTTP GET to the SubscribeURL.
type HTTPSubscriptionConfirmer struct {
	client *http.Client
}

// NewHTTPSubscriptionConfirmer creates a new confirmer with the given HTTP client.
func NewHTTPSubscriptionConfirmer(client *http.Client) *HTTPSubscriptionConfirmer {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPSubscriptionConfirmer{client: client}
}

// ConfirmSubscription fetches the SubscribeURL to confirm the SNS subscription.
func (c *HTTPSubscriptionConfirmer) ConfirmSubscription(ctx context.Context, subscribeURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subscribeURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

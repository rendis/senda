package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	seswebhook "github.com/rendis/senda/internal/adapter/ses/webhook"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

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
	processor                 *service.EventProcessor
	verifier                  SNSVerifier
	confirmer                 SubscriptionConfirmer
	translator                seswebhook.Translator
	replayStore               port.SNSReplayStore
	replayWindow              time.Duration
	logger                    *slog.Logger
	expectedTopicArn          string
	expectedAccountID         string
	skipSignatureVerification bool
	bindingMu                 sync.RWMutex
	registeredTopicArns       map[string]struct{}
}

// SESWebhookHandlerOption configures optional behavior for the SES webhook handler.
type SESWebhookHandlerOption func(*SESWebhookHandler)

// WithSkipSignatureVerification disables SNS signature verification.
// Intended only for isolated test harnesses that replay trusted envelopes.
func WithSkipSignatureVerification(skip bool) SESWebhookHandlerOption {
	return func(h *SESWebhookHandler) { h.skipSignatureVerification = skip }
}

// WithSNSBinding binds inbound messages to an exact TopicArn.
func WithSNSBinding(expectedTopicArn, _ string) SESWebhookHandlerOption {
	return func(h *SESWebhookHandler) {
		h.RegisterSNSBinding(expectedTopicArn)
	}
}

// WithExpectedSNSDestination configures the strict security-perimeter SNS destination.
// Unlike WithSNSBinding, mismatches are treated as invalid requests (400) instead of
// late-bound binding denials (403).
func WithExpectedSNSDestination(topicArn, accountID string) SESWebhookHandlerOption {
	return func(h *SESWebhookHandler) {
		h.expectedTopicArn = strings.TrimSpace(topicArn)
		h.expectedAccountID = strings.TrimSpace(accountID)
		if h.expectedTopicArn != "" {
			h.RegisterSNSBinding(h.expectedTopicArn)
		}
	}
}

// WithSNSReplayStore enables persistent replay protection for SNS envelopes.
func WithSNSReplayStore(store port.SNSReplayStore, replayWindow time.Duration) SESWebhookHandlerOption {
	return func(h *SESWebhookHandler) {
		h.replayStore = store
		h.replayWindow = replayWindow
	}
}

// NewSESWebhookHandler creates a new SES webhook handler.
func NewSESWebhookHandler(
	processor *service.EventProcessor,
	verifier SNSVerifier,
	confirmer SubscriptionConfirmer,
	logger *slog.Logger,
	opts ...SESWebhookHandlerOption,
) *SESWebhookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &SESWebhookHandler{
		processor:           processor,
		verifier:            verifier,
		confirmer:           confirmer,
		translator:          seswebhook.NewTranslator(),
		logger:              logger,
		registeredTopicArns: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterSNSBinding adds a TopicArn to the exact-match allowlist for inbound SNS.
func (h *SESWebhookHandler) RegisterSNSBinding(topicArn string) {
	topicArn = strings.TrimSpace(topicArn)
	if topicArn == "" {
		return
	}
	h.bindingMu.Lock()
	defer h.bindingMu.Unlock()
	if h.registeredTopicArns == nil {
		h.registeredTopicArns = make(map[string]struct{})
	}
	h.registeredTopicArns[topicArn] = struct{}{}
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
	if h.verifier != nil && !h.skipSignatureVerification {
		if err := h.verifier.Verify(body); err != nil {
			h.logger.WarnContext(ctx, "SNS signature verification failed", "error", err)
			return c.NoContent(http.StatusForbidden)
		}
	}

	parsed, err := h.translator.Translate(body)
	if err != nil {
		switch {
		case seswebhook.IsBadRequest(err):
			h.logger.WarnContext(ctx, "invalid SNS webhook payload", "error", err)
			return c.NoContent(http.StatusBadRequest)
		case seswebhook.IsMalformedNotification(err):
			h.logger.ErrorContext(ctx, "failed to parse SES notification", "error", err)
			return c.NoContent(http.StatusOK)
		default:
			h.logger.ErrorContext(ctx, "failed to translate SES webhook payload", "error", err)
			return c.NoContent(http.StatusOK)
		}
	}

	if err := h.validateExpectedSNSDestination(parsed.TopicArn); err != nil {
		h.logger.WarnContext(ctx, "SNS message rejected by topic policy",
			"message_id", parsed.MessageID,
			"topic_arn", parsed.TopicArn,
			"error", err,
		)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.validateSNSBinding(parsed.TopicArn); err != nil {
		h.logger.WarnContext(ctx, "SNS message rejected by binding policy",
			"message_id", parsed.MessageID,
			"topic_arn", parsed.TopicArn,
			"error", err,
		)
		return c.NoContent(http.StatusForbidden)
	}

	if ok, err := h.claimSNSReplay(ctx, c, parsed); err != nil {
		return err
	} else if !ok {
		return nil
	}

	switch parsed.Kind {
	case seswebhook.KindSubscriptionConfirmation:
		return h.handleSubscriptionConfirmation(ctx, c, parsed)
	case seswebhook.KindNotification:
		return h.handleNotification(ctx, c, parsed)
	default:
		if parsed.NotificationType == "Send" {
			h.logger.DebugContext(ctx, "ignoring SES Send notification (no provider event mapping needed)",
				"notification_type", parsed.NotificationType,
			)
		} else {
			h.logger.WarnContext(ctx, "unhandled SES notification type",
				"notification_type", parsed.NotificationType,
			)
		}
		return c.NoContent(http.StatusOK)
	}
}

// handleSubscriptionConfirmation auto-confirms the SNS subscription.
func (h *SESWebhookHandler) handleSubscriptionConfirmation(ctx context.Context, c *echo.Context, msg *seswebhook.ParsedMessage) error {
	h.logger.InfoContext(ctx, "confirming SNS subscription",
		"topic_arn", msg.TopicArn,
		"subscribe_url", redactURL(msg.SubscribeURL),
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

func (h *SESWebhookHandler) validateSNSBinding(topicArn string) error {
	h.bindingMu.RLock()
	defer h.bindingMu.RUnlock()

	if len(h.registeredTopicArns) == 0 {
		if strings.TrimSpace(h.expectedTopicArn) == "" {
			return fmt.Errorf("SNS binding is not configured")
		}
		return nil
	}
	if _, ok := h.registeredTopicArns[topicArn]; !ok {
		return fmt.Errorf("unexpected TopicArn")
	}
	return nil
}

func (h *SESWebhookHandler) validateExpectedSNSDestination(topicArn string) error {
	expectedTopicArn := strings.TrimSpace(h.expectedTopicArn)
	if expectedTopicArn == "" {
		return nil
	}
	if topicArn != expectedTopicArn {
		return fmt.Errorf("unexpected SNS TopicArn %q", topicArn)
	}

	expectedAccountID := strings.TrimSpace(h.expectedAccountID)
	if expectedAccountID == "" {
		var err error
		expectedAccountID, err = snsAccountIDFromArn(expectedTopicArn)
		if err != nil {
			return err
		}
	}

	actualAccountID, err := snsAccountIDFromArn(topicArn)
	if err != nil {
		return err
	}
	if actualAccountID != expectedAccountID {
		return fmt.Errorf("unexpected SNS account %q", actualAccountID)
	}

	return nil
}

func (h *SESWebhookHandler) claimSNSReplay(ctx context.Context, c *echo.Context, msg *seswebhook.ParsedMessage) (bool, error) {
	if msg == nil {
		return false, c.NoContent(http.StatusBadRequest)
	}
	if h.replayStore == nil {
		h.logger.ErrorContext(ctx, "SNS replay policy is not configured",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageID,
		)
		return false, c.NoContent(http.StatusInternalServerError)
	}
	if h.replayWindow <= 0 {
		h.logger.ErrorContext(ctx, "SNS replay window is invalid",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageID,
			"replay_window", h.replayWindow,
		)
		return false, c.NoContent(http.StatusInternalServerError)
	}
	if strings.TrimSpace(msg.MessageID) == "" {
		h.logger.WarnContext(ctx, "SNS replay envelope missing MessageId",
			"topic_arn", msg.TopicArn,
		)
		return false, c.NoContent(http.StatusBadRequest)
	}

	messageTimestamp, err := parseSNSReplayTimestamp(msg.Timestamp)
	if err != nil {
		h.logger.WarnContext(ctx, "SNS replay envelope has invalid Timestamp",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageID,
			"timestamp", msg.Timestamp,
			"error", err,
		)
		return false, c.NoContent(http.StatusBadRequest)
	}

	decision, err := h.replayStore.Claim(ctx, msg.TopicArn, msg.MessageID, messageTimestamp, h.replayWindow)
	if err != nil {
		h.logger.ErrorContext(ctx, "SNS replay claim failed",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageID,
			"error", err,
		)
		return false, c.NoContent(http.StatusInternalServerError)
	}

	switch decision {
	case port.SNSReplayDecisionAccepted:
		return true, nil
	case port.SNSReplayDecisionDuplicate:
		h.logger.WarnContext(ctx, "duplicate SNS replay rejected",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageID,
		)
		return false, c.NoContent(http.StatusOK)
	case port.SNSReplayDecisionStale:
		h.logger.WarnContext(ctx, "stale SNS replay rejected",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageID,
			"timestamp", messageTimestamp,
			"replay_window", h.replayWindow,
		)
		return false, c.NoContent(http.StatusOK)
	default:
		h.logger.ErrorContext(ctx, "unknown SNS replay decision",
			"topic_arn", msg.TopicArn,
			"message_id", msg.MessageID,
			"decision", decision,
		)
		return false, c.NoContent(http.StatusInternalServerError)
	}
}

// handleNotification processes an SES event notification.
func (h *SESWebhookHandler) handleNotification(ctx context.Context, c *echo.Context, msg *seswebhook.ParsedMessage) error {
	// Process the event (updates status, suppression, webhooks).
	if err := h.processor.Process(ctx, msg.Event); err != nil {
		h.logger.ErrorContext(ctx, "failed to process SES event",
			"notification_type", msg.NotificationType,
			"provider_message_id", msg.Event.ProviderMessageID,
			"error", err,
		)
		// Return 200 to prevent SNS retries — we log the error.
	}

	return c.NoContent(http.StatusOK)
}

func parseSNSReplayTimestamp(value string) (time.Time, error) {
	ts := strings.TrimSpace(value)
	if ts == "" {
		return time.Time{}, fmt.Errorf("missing timestamp")
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err == nil {
		return parsed.UTC(), nil
	}
	parsed, err = time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func snsAccountIDFromArn(topicArn string) (string, error) {
	parts := strings.Split(topicArn, ":")
	if len(parts) < 6 || parts[0] != "arn" || parts[1] != "aws" || parts[2] != "sns" {
		return "", fmt.Errorf("invalid SNS TopicArn %q", topicArn)
	}
	if parts[4] == "" {
		return "", fmt.Errorf("invalid SNS TopicArn %q", topicArn)
	}
	return parts[4], nil
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
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("SNS subscription confirmation failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

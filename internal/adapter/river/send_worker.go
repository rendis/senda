package river

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	goriver "github.com/riverqueue/river"

	gmailadapter "github.com/senda-app/senda/internal/adapter/gmail"
	sesadapter "github.com/senda-app/senda/internal/adapter/ses"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/metrics"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/tracking"
)

// AdapterSenderFactory creates a provider sender from a resolved adapter config.
type AdapterSenderFactory func(ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte) (port.EmailSender, error)

// DefaultAdapterSenderFactory builds provider-specific senders from adapter configs.
func DefaultAdapterSenderFactory(ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte) (port.EmailSender, error) {
	switch adapter.AdapterType {
	case domain.AdapterTypeSES:
		var cfg sesadapter.Config
		if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
			return nil, fmt.Errorf("%w: unmarshal SES config: %v", domain.ErrValidation, err)
		}
		if cfg.Region == "" {
			return nil, fmt.Errorf("%w: missing SES region", domain.ErrValidation)
		}
		return sesadapter.NewAdapterFromConfig(ctx, cfg)
	case domain.AdapterTypeGmail:
		var cfg gmailadapter.GmailConfig
		if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
			return nil, fmt.Errorf("%w: unmarshal Gmail config: %v", domain.ErrValidation, err)
		}
		if cfg.OAuthClientID == "" || cfg.OAuthClientSecret == "" || cfg.RefreshToken == "" {
			return nil, fmt.Errorf("%w: missing required Gmail OAuth fields", domain.ErrValidation)
		}
		return gmailadapter.NewAdapterFromConfig(ctx, cfg)
	default:
		return nil, fmt.Errorf("%w: unsupported adapter type %s", domain.ErrValidation, adapter.AdapterType)
	}
}

// senderCacheTTL is the duration a cached sender is considered valid.
const senderCacheTTL = 10 * time.Minute

// cachedSender holds a sender and the time it was created for TTL expiry.
type cachedSender struct {
	sender    port.EmailSender
	createdAt time.Time
}

// errorClassifier is implemented by adapters that can classify send errors
// as permanent (non-retryable) or transient.
type errorClassifier interface {
	IsPermanentSendError(error) bool
}

// SendWorker processes email send jobs.
type SendWorker struct {
	goriver.WorkerDefaults[SendJobArgs]

	emailStore      port.EmailStore
	compiler        port.TemplateCompiler
	renderer        port.VariableRenderer
	rateLimiter     port.RateLimiter
	sender          port.EmailSender
	adapterStore    port.AdapterStore
	crypto          port.Crypto
	senderFactory   AdapterSenderFactory
	trackingBaseURL string
	senderCache     sync.Map // uuid.UUID -> *cachedSender
}

// NewSendWorker creates a new send worker with all dependencies.
func NewSendWorker(
	emailStore port.EmailStore,
	compiler port.TemplateCompiler,
	renderer port.VariableRenderer,
	rateLimiter port.RateLimiter,
	sender port.EmailSender,
	opts ...SendWorkerOption,
) *SendWorker {
	w := &SendWorker{
		emailStore:  emailStore,
		compiler:    compiler,
		renderer:    renderer,
		rateLimiter: rateLimiter,
		sender:      sender,
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// SendWorkerOption configures optional SendWorker dependencies.
type SendWorkerOption func(*SendWorker)

// WithTrackingBaseURL sets the base URL for open-tracking pixels.
func WithTrackingBaseURL(url string) SendWorkerOption {
	return func(w *SendWorker) { w.trackingBaseURL = url }
}

// WithAdapterRuntime enables adapter-driven sender resolution when no static sender is configured.
func WithAdapterRuntime(adapterStore port.AdapterStore, crypto port.Crypto, senderFactory AdapterSenderFactory) SendWorkerOption {
	return func(w *SendWorker) {
		w.adapterStore = adapterStore
		w.crypto = crypto
		w.senderFactory = senderFactory
	}
}

// Work processes a single email send job.
func (w *SendWorker) Work(ctx context.Context, job *goriver.Job[SendJobArgs]) error {
	args := job.Args

	// 1. Fetch email by tracking ID.
	email, err := w.emailStore.GetByTrackingID(ctx, args.TrackingID)
	if err != nil {
		return goriver.JobCancel(fmt.Errorf("send: email not found for tracking_id=%s: %w", args.TrackingID, err))
	}

	// 1b. Idempotency guard: skip already-terminal emails.
	if email.Status == domain.StatusSent || email.Status == domain.StatusFailed || email.Status == domain.StatusSuppressed {
		return goriver.JobCancel(fmt.Errorf("email already in terminal state: %s", email.Status))
	}
	// If processing with provider ID set, it was sent but status update failed -- just update status.
	if email.Status == domain.StatusProcessing && email.ProviderMessageID != nil && *email.ProviderMessageID != "" {
		if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusSent, domain.StatusProcessing); err != nil {
			return fmt.Errorf("send: recover sent status: %w", err)
		}
		return nil
	}

	// 2. Rate limiter check (before status transition so email stays "queued" if denied).
	allowed, err := w.rateLimiter.TryAcquire(ctx, args.AdapterID)
	if err != nil {
		return fmt.Errorf("send: rate limiter error: %w", err)
	}
	if !allowed {
		// Snooze and retry later without counting as an attempt.
		return goriver.JobSnooze(5 * time.Second)
	}

	// 3. Mark as processing + add event.
	now := time.Now().UTC()
	if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusProcessing, domain.StatusQueued); err != nil {
		if errors.Is(err, domain.ErrStatusConflict) {
			return goriver.JobCancel(fmt.Errorf("send: email %s already claimed (status conflict)", email.ID))
		}
		return fmt.Errorf("send: update status to processing: %w", err)
	}
	email.Status = domain.StatusProcessing // Update in-memory state for downstream callers (failPermanently)
	if err := w.emailStore.AddEvent(ctx, &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.EventTypeProcessing,
		OccurredAt: now,
		CreatedAt:  now,
	}); err != nil {
		slog.Error("send_worker: failed to add processing event", "email_id", email.ID, "error", err)
	}

	// 4. Render MJML body with variables.
	renderedBody, err := w.renderer.Render(email.BodyMJML, email.InjectorsSnapshot, email.VariablesSnapshot)
	if err != nil {
		return w.failPermanently(ctx, email, fmt.Errorf("send: render body: %w", err))
	}

	// 5. Compile rendered MJML to HTML.
	bodyHTML, err := w.compiler.Compile(ctx, renderedBody)
	if err != nil {
		return w.failPermanently(ctx, email, fmt.Errorf("send: compile mjml: %w", err))
	}

	// 6. Inject open-tracking pixel if email has it enabled (denormalized from workspace at enqueue time).
	if w.trackingBaseURL != "" && email.OpenTrackingEnabled {
		bodyHTML = tracking.InjectOpenPixel(bodyHTML, w.trackingBaseURL, email.TrackingID)
	}

	// 7. Build outgoing email.
	outgoing := &port.OutgoingEmail{
		From:       port.EmailAddress{Name: email.FromName, Address: email.FromEmail},
		To:         port.EmailAddress{Address: email.RecipientEmail},
		Subject:    email.SubjectRendered,
		BodyHTML:   bodyHTML,
		TrackingID: email.TrackingID,
		Headers:    map[string]string{"X-Senda-Tracking-ID": email.TrackingID},
	}

	// CC / BCC
	for _, cc := range email.CC {
		outgoing.CC = append(outgoing.CC, port.EmailAddress{Address: cc})
	}
	for _, bcc := range email.BCC {
		outgoing.BCC = append(outgoing.BCC, port.EmailAddress{Address: bcc})
	}
	if email.ReplyTo != nil {
		outgoing.ReplyTo = &port.EmailAddress{Address: *email.ReplyTo}
	}

	// 8. Send via provider adapter.
	sender, err := w.resolveSender(ctx, email.AdapterID)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) || errors.Is(err, domain.ErrNotFound) {
			return w.failPermanently(ctx, email, fmt.Errorf("send: resolve sender: %w", err))
		}
		return fmt.Errorf("send: resolve sender: %w", err)
	}
	sendStart := time.Now()
	providerMsgID, err := sender.Send(ctx, outgoing)
	metrics.EmailSendDuration.WithLabelValues(sender.Name()).Observe(time.Since(sendStart).Seconds())
	if err != nil {
		return w.handleSendError(ctx, email, sender, job.Attempt, err)
	}

	// 9. Persist provider message ID for webhook event matching.
	if providerMsgID != "" {
		if err := w.emailStore.SetProviderMessageID(ctx, email.ID, providerMsgID); err != nil {
			slog.Error("send_worker: failed to set provider_message_id", "email_id", email.ID, "error", err)
		}
	}

	// 10. Success: update status to sent, add event.
	if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusSent, domain.StatusProcessing); err != nil {
		return fmt.Errorf("send: update status to sent: %w", err)
	}
	metrics.EmailsSent.WithLabelValues("sent", sender.Name(), email.TenantID.String(), email.WorkspaceID.String()).Inc()
	sentAt := time.Now().UTC()
	if err := w.emailStore.AddEvent(ctx, &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.EventTypeSent,
		OccurredAt: sentAt,
		Metadata:   map[string]any{"provider_message_id": providerMsgID},
		CreatedAt:  sentAt,
	}); err != nil {
		slog.Error("send_worker: failed to add sent event", "email_id", email.ID, "error", err)
	}

	return nil
}

// NextRetry implements exponential backoff: 60s * 2^attempt.
func (w *SendWorker) NextRetry(job *goriver.Job[SendJobArgs]) time.Time {
	backoff := time.Duration(60*(1<<uint(job.Attempt-1))) * time.Second
	return time.Now().Add(backoff)
}

// handleSendError determines if an error is transient or permanent.
func (w *SendWorker) handleSendError(ctx context.Context, email *domain.Email, sender port.EmailSender, attempt int, sendErr error) error {
	if isPermanentSendError(sender, sendErr) {
		return w.failPermanently(ctx, email, sendErr)
	}
	// Transient error: let River retry with exponential backoff.
	retryAt := time.Now().Add(time.Duration(60*(1<<uint(attempt-1))) * time.Second)
	if err := w.emailStore.UpdateRetry(ctx, email.ID, attempt, &retryAt); err != nil {
		slog.Error("send_worker: failed to update retry", "email_id", email.ID, "attempt", attempt, "error", err)
	}
	return sendErr
}

// failPermanently marks the email as failed and cancels the job.
func (w *SendWorker) failPermanently(ctx context.Context, email *domain.Email, reason error) error {
	metrics.EmailsFailed.Inc()
	if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusFailed, email.Status); err != nil {
		slog.Error("send_worker: failed to update status to failed", "email_id", email.ID, "error", err)
	}
	now := time.Now().UTC()
	if err := w.emailStore.AddEvent(ctx, &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.EventTypeFailed,
		OccurredAt: now,
		Metadata:   map[string]any{"error": reason.Error()},
		CreatedAt:  now,
	}); err != nil {
		slog.Error("send_worker: failed to add failure event", "email_id", email.ID, "error", err)
	}
	return goriver.JobCancel(reason)
}

func (w *SendWorker) resolveSender(ctx context.Context, adapterID uuid.UUID) (port.EmailSender, error) {
	if w.sender != nil {
		return w.sender, nil
	}
	if adapterID == uuid.Nil {
		return nil, fmt.Errorf("%w: missing adapter id", domain.ErrValidation)
	}
	if w.adapterStore == nil || w.crypto == nil {
		return nil, fmt.Errorf("%w: adapter runtime sender is not configured", domain.ErrValidation)
	}

	// Check sender cache before creating a new one.
	if cached, ok := w.senderCache.Load(adapterID); ok {
		cs := cached.(*cachedSender)
		if time.Since(cs.createdAt) < senderCacheTTL {
			return cs.sender, nil
		}
		w.senderCache.Delete(adapterID) // expired
	}

	factory := w.senderFactory
	if factory == nil {
		factory = DefaultAdapterSenderFactory
	}

	adapter, err := w.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	decryptedConfig, err := w.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return nil, fmt.Errorf("%w: decrypt adapter config: %v", domain.ErrValidation, err)
	}
	sender, err := factory(ctx, adapter, decryptedConfig)
	if err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, fmt.Errorf("%w: adapter sender factory returned nil for %s", domain.ErrValidation, adapter.AdapterType)
	}

	w.senderCache.Store(adapterID, &cachedSender{sender: sender, createdAt: time.Now()})
	return sender, nil
}

// isPermanentSendError checks if the error indicates a permanent failure.
// It delegates to the sender's adapter-specific classifier (SES, Gmail).
// If the sender doesn't implement errorClassifier, the error is treated as
// transient so River will retry with backoff. This avoids brittle string
// matching -- all real permanent error detection lives in the typed adapters.
func isPermanentSendError(sender port.EmailSender, err error) bool {
	if classifier, ok := sender.(errorClassifier); ok {
		return classifier.IsPermanentSendError(err)
	}
	return false
}

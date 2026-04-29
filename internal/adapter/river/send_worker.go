package river

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	goriver "github.com/riverqueue/river"

	gmailadapter "github.com/rendis/senda/internal/adapter/gmail"
	sesadapter "github.com/rendis/senda/internal/adapter/ses"
	smtpadapter "github.com/rendis/senda/internal/adapter/smtp"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/metrics"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/tracking"
)

// DefaultAdapterSenderFactory builds provider-specific senders from adapter configs.
func DefaultAdapterSenderFactory(ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte) (port.EmailSender, error) {
	return NewAdapterSenderFactory(smtpadapter.CleartextAuthPolicy{})(ctx, adapter, decryptedConfig)
}

// NewAdapterSenderFactory builds provider-specific senders using runtime adapter policy.
func NewAdapterSenderFactory(smtpPolicy smtpadapter.CleartextAuthPolicy) port.SenderFactory {
	return func(ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte) (port.EmailSender, error) {
		switch adapter.AdapterType {
		case domain.AdapterTypeSES:
			var cfg sesadapter.Config
			if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
				return nil, fmt.Errorf("%w: unmarshal SES config: %w", domain.ErrValidation, err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("%w: %w", domain.ErrValidation, err)
			}
			return sesadapter.NewAdapterFromConfig(ctx, cfg)
		case domain.AdapterTypeGmail:
			var cfg gmailadapter.GmailConfig
			if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
				return nil, fmt.Errorf("%w: unmarshal Gmail config: %w", domain.ErrValidation, err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("%w: %w", domain.ErrValidation, err)
			}
			return gmailadapter.NewAdapterFromConfig(ctx, cfg)
		case domain.AdapterTypeSMTP:
			var cfg smtpadapter.Config
			if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
				return nil, fmt.Errorf("%w: unmarshal SMTP config: %w", domain.ErrValidation, err)
			}
			return smtpadapter.NewAdapterFromConfigWithPolicy(cfg, smtpPolicy)
		default:
			return nil, fmt.Errorf("%w: unsupported adapter type %s", domain.ErrValidation, adapter.AdapterType)
		}
	}
}

// senderCacheTTL is the duration a cached sender is considered valid.
const senderCacheTTL = 10 * time.Minute
const defaultRateLimitBurstSize = 4
const defaultRateLimitReservationTTL = 10 * time.Minute
const defaultRateLimitCleanupInterval = time.Minute

// staleProcessingThreshold is how long an email can sit in StatusProcessing
// without a ProviderMessageID before it is considered crashed (not in-flight).
const staleProcessingThreshold = 10 * time.Minute

// cachedSender holds a sender and the time it was created for TTL expiry.
type cachedSender struct {
	sender    port.EmailSender
	createdAt time.Time
}

type rateLimitReservation struct {
	mu          sync.Mutex
	available   int
	lastTouched time.Time
	refs        atomic.Int32
	deleting    atomic.Bool
}

// errorClassifier is implemented by adapters that can classify send errors
// as permanent (non-retryable) or transient.
type errorClassifier interface {
	IsPermanentSendError(error) bool
}

// SendWorker processes email send jobs.
type SendWorker struct {
	goriver.WorkerDefaults[SendJobArgs]

	emailStore               port.EmailStore
	compiler                 port.TemplateCompiler
	renderer                 port.VariableRenderer
	rateLimiter              port.RateLimiter
	sender                   port.EmailSender
	adapterStore             port.AdapterStore
	crypto                   port.Crypto
	senderFactory            port.SenderFactory
	trackingBaseURL          string
	rateLimitBurstSize       int
	rateLimitReservationTTL  time.Duration
	rateLimitCleanupInterval time.Duration
	now                      func() time.Time
	senderCache              sync.Map // uuid.UUID -> *cachedSender
	rateLimits               sync.Map // uuid.UUID -> *rateLimitReservation
	rateLimitCleanupRunning  atomic.Bool
	lastRateLimitCleanupUnix atomic.Int64
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
		emailStore:               emailStore,
		compiler:                 compiler,
		renderer:                 renderer,
		rateLimiter:              rateLimiter,
		sender:                   sender,
		rateLimitBurstSize:       defaultRateLimitBurstSize,
		rateLimitReservationTTL:  defaultRateLimitReservationTTL,
		rateLimitCleanupInterval: defaultRateLimitCleanupInterval,
		now:                      func() time.Time { return time.Now().UTC() },
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
func WithAdapterRuntime(adapterStore port.AdapterStore, crypto port.Crypto, senderFactory port.SenderFactory) SendWorkerOption {
	return func(w *SendWorker) {
		w.adapterStore = adapterStore
		w.crypto = crypto
		w.senderFactory = senderFactory
	}
}

// WithRateLimitBurstSize overrides the local reservation size used per adapter.
func WithRateLimitBurstSize(size int) SendWorkerOption {
	return func(w *SendWorker) {
		if size > 0 {
			w.rateLimitBurstSize = size
		}
	}
}

// Work processes a single email send job.
func (w *SendWorker) Work(ctx context.Context, job *goriver.Job[SendJobArgs]) error { //nolint:gocognit,gocyclo,funlen // multi-step email send pipeline
	args := job.Args
	recoveringProcessingSend := false

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
	// If processing with no provider ID, the worker may have crashed before completing
	// the send pipeline. For recent timestamps we resume the delivery attempt; for
	// stale ones we still fail permanently to avoid endlessly retrying ambiguous sends.
	if email.Status == domain.StatusProcessing && (email.ProviderMessageID == nil || *email.ProviderMessageID == "") {
		if time.Since(email.UpdatedAt) > staleProcessingThreshold {
			return w.failPermanently(ctx, email, fmt.Errorf("send: email stuck in processing, likely crashed during send"))
		}
		recoveringProcessingSend = true
	}

	// 2. Rate limiter check (before status transition so email stays "queued" if denied).
	allowed, err := w.acquireRateLimitToken(ctx, args.AdapterID)
	if err != nil {
		return fmt.Errorf("send: rate limiter error: %w", err)
	}
	if !allowed {
		// Snooze and retry later without counting as an attempt.
		return goriver.JobSnooze(5 * time.Second)
	}

	// 3. Mark as processing + add event.
	now := time.Now().UTC()
	if !recoveringProcessingSend {
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
			slog.Error("send_worker: failed to add processing event", append(emailLogAttrs(email), "error", err)...)
		}
	}

	// 4. Load cold payload only when the worker actually needs to render/send.
	payload, err := w.emailStore.GetPayload(ctx, email.ID)
	if err != nil {
		return w.failPermanently(ctx, email, fmt.Errorf("send: load email payload: %w", err))
	}

	// 5. Render MJML body with variables.
	renderedBody, err := w.renderer.Render(payload.BodyMJML, payload.InjectorsSnapshot, payload.VariablesSnapshot)
	if err != nil {
		return w.failPermanently(ctx, email, fmt.Errorf("send: render body: %w", err))
	}

	// 6. Compile rendered MJML to HTML.
	bodyHTML, err := w.compiler.Compile(ctx, renderedBody)
	if err != nil {
		return w.failPermanently(ctx, email, fmt.Errorf("send: compile mjml: %w", err))
	}

	// 7. Inject open-tracking pixel if email has it enabled (denormalized from workspace at enqueue time).
	if w.trackingBaseURL != "" && email.OpenTrackingEnabled {
		bodyHTML = tracking.InjectOpenPixel(bodyHTML, w.trackingBaseURL, email.TrackingID)
	}

	// 8. Build outgoing email.
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

	// 9. Send via provider adapter.
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
		return w.handleSendError(ctx, email, sender, job.Attempt, job.MaxAttempts, err)
	}

	// 10. Persist provider message ID for webhook event matching.
	if providerMsgID != "" {
		if err := w.emailStore.SetProviderMessageID(ctx, email.ID, providerMsgID); err != nil {
			return fmt.Errorf("send: set provider_message_id: %w", err)
		}
	}

	// 11. Success: update status to sent, add event.
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
		slog.Error("send_worker: failed to add sent event", append(emailLogAttrs(email), "provider_message_id", providerMsgID, "error", err)...)
	}

	slog.Info("send_worker: email sent", append(emailLogAttrs(email),
		"provider", sender.Name(),
		"provider_message_id", providerMsgID,
	)...)

	return nil
}

func (w *SendWorker) acquireRateLimitToken(ctx context.Context, adapterID uuid.UUID) (bool, error) {
	if w.rateLimiter == nil {
		return true, nil
	}

	now := w.now()
	for {
		stateAny, _ := w.rateLimits.LoadOrStore(adapterID, &rateLimitReservation{})
		state := stateAny.(*rateLimitReservation)
		state.refs.Add(1)

		if state.deleting.Load() {
			state.refs.Add(-1)
			continue
		}

		state.mu.Lock()
		if state.deleting.Load() {
			state.mu.Unlock()
			state.refs.Add(-1)
			continue
		}

		if state.lastTouched.IsZero() {
			state.lastTouched = now
		}
		if w.rateLimitReservationTTL > 0 && now.Sub(state.lastTouched) >= w.rateLimitReservationTTL {
			state.available = 0
		}

		if state.available > 0 {
			state.available--
			state.lastTouched = now
			state.mu.Unlock()
			state.refs.Add(-1)
			return true, nil
		}

		reserved, err := w.rateLimiter.AcquireBurst(ctx, adapterID, w.rateLimitBurstSize)
		if err != nil {
			state.mu.Unlock()
			state.refs.Add(-1)
			return false, err
		}
		if reserved < 1 {
			state.mu.Unlock()
			state.refs.Add(-1)
			return false, nil
		}

		state.available = reserved - 1
		state.lastTouched = now
		state.mu.Unlock()
		state.refs.Add(-1)
		w.maybeScheduleRateLimitCleanup(now)
		return true, nil
	}
}

func (w *SendWorker) maybeScheduleRateLimitCleanup(now time.Time) {
	if w.rateLimitReservationTTL <= 0 || w.rateLimitCleanupInterval <= 0 {
		return
	}

	lastUnix := w.lastRateLimitCleanupUnix.Load()
	if lastUnix != 0 && now.Sub(time.Unix(0, lastUnix)) < w.rateLimitCleanupInterval {
		return
	}
	if !w.rateLimitCleanupRunning.CompareAndSwap(false, true) {
		return
	}

	w.lastRateLimitCleanupUnix.Store(now.UnixNano())
	go func(cleanupAt time.Time) {
		defer w.rateLimitCleanupRunning.Store(false)
		w.cleanupExpiredRateLimitReservations(cleanupAt)
	}(now)
}

func (w *SendWorker) cleanupExpiredRateLimitReservations(now time.Time) {
	w.rateLimits.Range(func(key, value any) bool {
		reservation := value.(*rateLimitReservation)
		if !reservation.deleting.CompareAndSwap(false, true) {
			return true
		}
		if reservation.refs.Load() != 0 {
			reservation.deleting.Store(false)
			return true
		}
		reservation.mu.Lock()
		stale := w.rateLimitReservationTTL > 0 &&
			!reservation.lastTouched.IsZero() &&
			now.Sub(reservation.lastTouched) >= w.rateLimitReservationTTL
		refs := reservation.refs.Load()
		reservation.mu.Unlock()
		if stale && refs == 0 {
			_ = w.rateLimits.CompareAndDelete(key, reservation)
			return true
		}
		reservation.deleting.Store(false)
		return true
	})
}

// sendBackoff returns the exponential backoff duration for a given attempt: 60s * 2^(attempt-1).
func sendBackoff(attempt int) time.Duration {
	return time.Duration(60*(1<<uint(attempt-1))) * time.Second
}

// NextRetry implements exponential backoff for River.
func (w *SendWorker) NextRetry(job *goriver.Job[SendJobArgs]) time.Time {
	return time.Now().Add(sendBackoff(job.Attempt))
}

// handleSendError determines if an error is transient or permanent.
// When attempt == maxAttempts, a transient error is escalated to a permanent
// failure so the email is marked Failed instead of being left in Processing.
func (w *SendWorker) handleSendError(ctx context.Context, email *domain.Email, sender port.EmailSender, attempt, maxAttempts int, sendErr error) error {
	if isPermanentSendError(sender, sendErr) {
		metrics.ProviderErrors.WithLabelValues(sender.Name(), "permanent").Inc()
		return w.failPermanently(ctx, email, sendErr)
	}
	// All transient errors are counted once, regardless of retry outcome.
	metrics.ProviderErrors.WithLabelValues(sender.Name(), "transient").Inc()
	// Final attempt: escalate to permanent failure.
	if attempt >= maxAttempts {
		return w.failPermanently(ctx, email, fmt.Errorf("send: transient error on final attempt (%d/%d): %w", attempt, maxAttempts, sendErr))
	}
	// Retries remaining: let River retry with exponential backoff.
	retryAt := time.Now().Add(sendBackoff(attempt))
	if err := w.emailStore.UpdateRetry(ctx, email.ID, attempt, &retryAt); err != nil {
		slog.Error("send_worker: failed to update retry", append(emailLogAttrs(email), "attempt", attempt, "error", err)...)
	}
	return sendErr
}

// failPermanently marks the email as failed and cancels the job.
func (w *SendWorker) failPermanently(ctx context.Context, email *domain.Email, reason error) error {
	metrics.EmailsFailed.Inc()
	if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusFailed, email.Status); err != nil {
		slog.Error("send_worker: failed to update status to failed", append(emailLogAttrs(email), "error", err)...)
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
		slog.Error("send_worker: failed to add failure event", append(emailLogAttrs(email), "error", err)...)
	}
	return goriver.JobCancel(reason)
}

func emailLogAttrs(email *domain.Email) []any {
	attrs := []any{
		"email_id", email.ID,
		"tracking_id", email.TrackingID,
		"tenant_id", email.TenantID,
		"workspace_id", email.WorkspaceID,
		"adapter_id", email.AdapterID,
		"from_email", email.FromEmail,
	}
	if email.SenderIdentityID != nil {
		attrs = append(attrs, "sender_identity_id", *email.SenderIdentityID)
	}
	return attrs
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
		return nil, fmt.Errorf("%w: decrypt adapter config: %w", domain.ErrValidation, err)
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

package river

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	goriver "github.com/riverqueue/river"

	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/tracking"
)

// SendWorker processes email send jobs.
type SendWorker struct {
	goriver.WorkerDefaults[SendJobArgs]

	emailStore      port.EmailStore
	domainStore     port.DomainStore
	crypto          port.Crypto
	compiler        port.TemplateCompiler
	renderer        port.VariableRenderer
	rateLimiter     port.RateLimiter
	sender          port.EmailSender
	wsStore         port.WorkspaceStore
	trackingBaseURL string
}

// NewSendWorker creates a new send worker with all dependencies.
func NewSendWorker(
	emailStore port.EmailStore,
	domainStore port.DomainStore,
	crypto port.Crypto,
	compiler port.TemplateCompiler,
	renderer port.VariableRenderer,
	rateLimiter port.RateLimiter,
	sender port.EmailSender,
	opts ...SendWorkerOption,
) *SendWorker {
	w := &SendWorker{
		emailStore:  emailStore,
		domainStore: domainStore,
		crypto:      crypto,
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

// WithWorkspaceStore sets the workspace store for open-tracking lookups.
func WithWorkspaceStore(ws port.WorkspaceStore) SendWorkerOption {
	return func(w *SendWorker) { w.wsStore = ws }
}

// WithTrackingBaseURL sets the base URL for open-tracking pixels.
func WithTrackingBaseURL(url string) SendWorkerOption {
	return func(w *SendWorker) { w.trackingBaseURL = url }
}

// Work processes a single email send job.
func (w *SendWorker) Work(ctx context.Context, job *goriver.Job[SendJobArgs]) error {
	args := job.Args

	// 1. Fetch email by tracking ID.
	email, err := w.emailStore.GetByTrackingID(ctx, args.TrackingID)
	if err != nil {
		return goriver.JobCancel(fmt.Errorf("send: email not found for tracking_id=%s: %w", args.TrackingID, err))
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
	if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusProcessing); err != nil {
		return fmt.Errorf("send: update status to processing: %w", err)
	}
	if err := w.emailStore.AddEvent(ctx, &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.StatusProcessing,
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

	// 6. Inject open-tracking pixel if workspace has it enabled.
	if w.trackingBaseURL != "" && w.wsStore != nil {
		ws, wsErr := w.wsStore.GetByID(ctx, email.WorkspaceID)
		if wsErr == nil && ws.OpenTrackingEnabled {
			bodyHTML = tracking.InjectOpenPixel(bodyHTML, w.trackingBaseURL, email.TrackingID)
		}
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

	// 8. DKIM config: look up domain by from_email, decrypt private key.
	dkimCfg, err := w.resolveDKIM(ctx, email.FromEmail)
	if err == nil && dkimCfg != nil {
		outgoing.DKIMConfig = dkimCfg
	}
	// If DKIM resolution fails, send without DKIM (not a fatal error).

	// 9. Send via provider adapter.
	providerMsgID, err := w.sender.Send(ctx, outgoing)
	if err != nil {
		return w.handleSendError(ctx, email, job.Attempt, err)
	}

	// 10. Persist provider message ID for webhook event matching.
	if providerMsgID != "" {
		if err := w.emailStore.SetProviderMessageID(ctx, email.ID, providerMsgID); err != nil {
			slog.Error("send_worker: failed to set provider_message_id", "email_id", email.ID, "error", err)
		}
	}

	// 11. Success: update status to sent, add event.
	if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusSent); err != nil {
		return fmt.Errorf("send: update status to sent: %w", err)
	}
	sentAt := time.Now().UTC()
	if err := w.emailStore.AddEvent(ctx, &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.StatusSent,
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

// resolveDKIM looks up the domain for the from_email and decrypts the DKIM key.
// TODO: Requires port.DomainStore.GetVerifiedByDomainName(ctx, workspaceID, domainName)
// Currently cannot resolve DKIM because GetPendingVerifications only returns pending domains.
// This will be fixed when the port method is added.
func (w *SendWorker) resolveDKIM(_ context.Context, _ string) (*port.DKIMConfig, error) {
	slog.Warn("send_worker: DKIM signing skipped, GetVerifiedByDomainName not yet available")
	return nil, nil
}

// handleSendError determines if an error is transient or permanent.
func (w *SendWorker) handleSendError(ctx context.Context, email *domain.Email, attempt int, sendErr error) error {
	if isPermanentSendError(sendErr) {
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
	if err := w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusFailed); err != nil {
		slog.Error("send_worker: failed to update status to failed", "email_id", email.ID, "error", err)
	}
	now := time.Now().UTC()
	if err := w.emailStore.AddEvent(ctx, &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.StatusFailed,
		OccurredAt: now,
		Metadata:   map[string]any{"error": reason.Error()},
		CreatedAt:  now,
	}); err != nil {
		slog.Error("send_worker: failed to add failure event", "email_id", email.ID, "error", err)
	}
	return goriver.JobCancel(reason)
}

// isPermanentSendError checks if the error indicates a permanent failure.
func isPermanentSendError(err error) bool {
	msg := err.Error()
	permanentPrefixes := []string{
		"invalid",
		"rejected",
		"blacklisted",
		"address not verified",
	}
	lower := strings.ToLower(msg)
	for _, prefix := range permanentPrefixes {
		if strings.Contains(lower, prefix) {
			return true
		}
	}
	return false
}

// extractDomain extracts the domain part from an email address.
func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

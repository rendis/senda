package river

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	goriver "github.com/riverqueue/river"

	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/netutil"
)

// HTTPClient abstracts HTTP requests for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// SSRFChecker is a function that returns true if the hostname should be blocked.
type SSRFChecker func(hostname string) bool

// WebhookWorker processes webhook delivery jobs.
type WebhookWorker struct {
	goriver.WorkerDefaults[WebhookJobArgs]

	webhookStore port.WebhookStore
	httpClient   HTTPClient
	ssrfChecker  SSRFChecker
}

// WebhookWorkerOption configures optional WebhookWorker settings.
type WebhookWorkerOption func(*WebhookWorker)

// WithSSRFChecker overrides the default SSRF host checker. Useful in tests to
// skip DNS resolution against test URLs like example.com.
func WithSSRFChecker(fn SSRFChecker) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.ssrfChecker = fn }
}

// NewWebhookWorker creates a new webhook delivery worker.
func NewWebhookWorker(webhookStore port.WebhookStore, httpClient HTTPClient, opts ...WebhookWorkerOption) *WebhookWorker {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	w := &WebhookWorker{
		webhookStore: webhookStore,
		httpClient:   httpClient,
		ssrfChecker:  netutil.IsPrivateOrReservedHost,
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Work processes a single webhook delivery job.
func (w *WebhookWorker) Work(ctx context.Context, job *goriver.Job[WebhookJobArgs]) error {
	args := job.Args

	// 1. Get webhook by ID, check IsActive.
	wh, err := w.webhookStore.GetByID(ctx, args.WebhookID)
	if err != nil {
		return goriver.JobCancel(fmt.Errorf("webhook: not found id=%s: %w", args.WebhookID, err))
	}
	if !wh.IsActive {
		return goriver.JobCancel(fmt.Errorf("webhook: endpoint %s is disabled", args.WebhookID))
	}

	// 2. SSRF guard — resolve webhook URL at delivery time to block DNS rebinding.
	parsed, err := url.Parse(wh.URL)
	if err != nil || w.ssrfChecker(parsed.Hostname()) {
		return goriver.JobCancel(fmt.Errorf("webhook URL blocked: %s", wh.URL))
	}

	// 3. Sign payload with HMAC-SHA256.
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := signPayload(wh.Secret, timestamp, args.Payload)

	// 4. POST to webhook URL with headers.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(args.Payload))
	if err != nil {
		return goriver.JobCancel(fmt.Errorf("webhook: invalid url %q: %w", wh.URL, err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Senda-Event", args.EventType)
	req.Header.Set("X-Senda-Signature", signature)
	req.Header.Set("X-Senda-Timestamp", timestamp)
	req.Header.Set("User-Agent", "Senda-Webhook/1.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		// Network error — transient, let River retry.
		return w.handleFailure(ctx, wh, fmt.Errorf("webhook: http error: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	// Read body (limited) for error reporting.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	// 5. Evaluate response status.
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Success — atomically reset failure counter.
		if wh.ConsecutiveFailures > 0 {
			if err := w.webhookStore.ResetFailureCount(ctx, wh.ID); err != nil {
				slog.Error("webhook_worker: failed to reset failure counter", "webhook_id", wh.ID, "error", err)
			}
		}
		return nil

	case resp.StatusCode == 429 || resp.StatusCode >= 500:
		// Transient — retry.
		return w.handleFailure(ctx, wh, fmt.Errorf("webhook: transient error status=%d", resp.StatusCode))

	default:
		// 4xx (not 429) — permanent failure.
		return w.handlePermanentFailure(ctx, wh, fmt.Errorf("webhook: permanent error status=%d", resp.StatusCode))
	}
}

// NextRetry implements exponential backoff for webhook retries.
func (w *WebhookWorker) NextRetry(job *goriver.Job[WebhookJobArgs]) time.Time {
	// 30s, 60s, 120s, 240s, 480s
	backoff := time.Duration(30*(1<<uint(job.Attempt-1))) * time.Second
	return time.Now().Add(backoff)
}

// handleFailure records a transient failure atomically and returns the error for retry.
func (w *WebhookWorker) handleFailure(ctx context.Context, wh *domain.Webhook, err error) error {
	failures, isActive, updateErr := w.webhookStore.IncrementFailureCount(ctx, wh.ID)
	if updateErr != nil {
		slog.Error("webhook_worker: failed to increment failure counter", "webhook_id", wh.ID, "error", updateErr)
	} else if !isActive {
		slog.Warn("webhook_worker: endpoint auto-disabled after consecutive failures", "webhook_id", wh.ID, "failures", failures)
	}
	return err
}

// handlePermanentFailure records a permanent failure atomically and cancels the job.
func (w *WebhookWorker) handlePermanentFailure(ctx context.Context, wh *domain.Webhook, err error) error {
	failures, isActive, updateErr := w.webhookStore.IncrementFailureCount(ctx, wh.ID)
	if updateErr != nil {
		slog.Error("webhook_worker: failed to increment failure counter", "webhook_id", wh.ID, "error", updateErr)
	} else if !isActive {
		slog.Warn("webhook_worker: endpoint auto-disabled after consecutive failures", "webhook_id", wh.ID, "failures", failures)
	}
	return goriver.JobCancel(err)
}

// signPayload creates the HMAC-SHA256 signature.
// Format: sha256=hex(HMAC-SHA256(secret, timestamp.payload))
func signPayload(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

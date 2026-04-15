package river

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	goriver "github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// --- Mock HTTP client ---

type mockHTTPClient struct {
	doFn  func(req *http.Request) (*http.Response, error)
	calls []*http.Request
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.calls = append(m.calls, req)
	if m.doFn != nil {
		return m.doFn(req)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

// --- Mock webhook store ---

type mockWebhookStore struct {
	getByIDFn               func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	updateFn                func(ctx context.Context, wh *domain.Webhook) error
	incrementFailureCountFn func(ctx context.Context, id uuid.UUID) (int, bool, error)
	resetFailureCountFn     func(ctx context.Context, id uuid.UUID) error

	updateCalls           []*domain.Webhook
	incrementFailureCalls []uuid.UUID
	resetFailureCalls     []uuid.UUID
}

func (m *mockWebhookStore) Create(ctx context.Context, wh *domain.Webhook) error { return nil }
func (m *mockWebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockWebhookStore) Update(ctx context.Context, wh *domain.Webhook) error {
	m.updateCalls = append(m.updateCalls, wh)
	if m.updateFn != nil {
		return m.updateFn(ctx, wh)
	}
	return nil
}
func (m *mockWebhookStore) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockWebhookStore) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Webhook], error) {
	return nil, nil
}
func (m *mockWebhookStore) GetActiveByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Webhook, error) {
	return nil, nil
}
func (m *mockWebhookStore) IncrementFailureCount(ctx context.Context, id uuid.UUID) (int, bool, error) {
	m.incrementFailureCalls = append(m.incrementFailureCalls, id)
	if m.incrementFailureCountFn != nil {
		return m.incrementFailureCountFn(ctx, id)
	}
	return 1, true, nil
}
func (m *mockWebhookStore) ResetFailureCount(ctx context.Context, id uuid.UUID) error {
	m.resetFailureCalls = append(m.resetFailureCalls, id)
	if m.resetFailureCountFn != nil {
		return m.resetFailureCountFn(ctx, id)
	}
	return nil
}

// noopSSRFChecker always allows the hostname — used in tests to avoid DNS lookups.
func noopSSRFChecker(_ string) bool { return false }

// --- Test helpers ---

func newTestWebhook() *domain.Webhook {
	return &domain.Webhook{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: uuid.Must(uuid.NewV7()),
		URL:         "https://hooks.example.com/senda",
		Secret:      "whsec_test_secret_key_123",
		Events:      []string{"email.delivered", "email.bounced"},
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

func makeWebhookJob(webhookID uuid.UUID, eventType string, payload []byte) *goriver.Job[WebhookJobArgs] {
	return &goriver.Job[WebhookJobArgs]{
		Args: WebhookJobArgs{
			WebhookID: webhookID,
			EventType: eventType,
			Payload:   payload,
		},
		JobRow: &rivertype.JobRow{
			Attempt: 1,
		},
	}
}

// --- Tests ---

func TestWebhookWorker_SuccessfulDelivery(t *testing.T) {
	wh := newTestWebhook()
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Webhook, error) {
			if id == wh.ID {
				return wh, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	httpClient := &mockHTTPClient{}
	worker := NewWebhookWorker(store, httpClient, WithSSRFChecker(noopSSRFChecker))

	payload := []byte(`{"email_id":"abc-123","event":"delivered"}`)
	job := makeWebhookJob(wh.ID, "email.delivered", payload)

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(httpClient.calls) != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", len(httpClient.calls))
	}
	req := httpClient.calls[0]
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if req.URL.String() != wh.URL {
		t.Errorf("url = %q, want %q", req.URL.String(), wh.URL)
	}
	if req.Header.Get("X-Senda-Event") != "email.delivered" {
		t.Errorf("X-Senda-Event = %q, want %q", req.Header.Get("X-Senda-Event"), "email.delivered")
	}
	if req.Header.Get("X-Senda-Signature") == "" {
		t.Error("expected X-Senda-Signature header")
	}
	if req.Header.Get("X-Senda-Timestamp") == "" {
		t.Error("expected X-Senda-Timestamp header")
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", req.Header.Get("Content-Type"), "application/json")
	}
}

func TestWebhookWorker_RedirectResponse_DoesNotFollowAndCancelsJob(t *testing.T) {
	targetHits := 0
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetServer.URL+"/target", http.StatusFound)
	}))
	defer redirectServer.Close()

	wh := newTestWebhook()
	wh.URL = redirectServer.URL
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Webhook, error) {
			if id == wh.ID {
				return wh, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	client := redirectServer.Client()
	worker := NewWebhookWorker(store, client, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte(`{"id":"abc"}`))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected redirect to fail, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Fatalf("expected JobCancelError for redirect, got %T: %v", err, err)
	}

	if targetHits != 0 {
		t.Fatalf("expected no follow-up request to redirect target, got %d", targetHits)
	}
}

func TestWebhookWorker_WebhookNotFound_CancelsJob(t *testing.T) {
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return nil, domain.ErrNotFound
		},
	}
	worker := NewWebhookWorker(store, &mockHTTPClient{}, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(uuid.Must(uuid.NewV7()), "email.delivered", []byte("{}"))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError, got %T: %v", err, err)
	}
}

func TestWebhookWorker_Disabled_CancelsJob(t *testing.T) {
	wh := newTestWebhook()
	wh.IsActive = false
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return wh, nil
		},
	}
	worker := NewWebhookWorker(store, &mockHTTPClient{}, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte("{}"))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError, got %T: %v", err, err)
	}
}

func TestWebhookWorker_ServerError_Retries(t *testing.T) {
	wh := newTestWebhook()
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return wh, nil
		},
	}
	httpClient := &mockHTTPClient{
		doFn: func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 500,
				Body:       io.NopCloser(strings.NewReader("internal server error")),
			}, nil
		},
	}
	worker := NewWebhookWorker(store, httpClient, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte("{}"))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected transient error, got nil")
	}

	// Should NOT be a cancel error — River should retry.
	var cancelErr *goriver.JobCancelError
	if errors.As(err, &cancelErr) {
		t.Error("expected transient error but got JobCancelError")
	}

	// Should have called IncrementFailureCount atomically.
	if len(store.incrementFailureCalls) == 0 {
		t.Fatal("expected IncrementFailureCount call")
	}
	if store.incrementFailureCalls[0] != wh.ID {
		t.Errorf("IncrementFailureCount called with %v, want %v", store.incrementFailureCalls[0], wh.ID)
	}
}

func TestWebhookWorker_429_Retries(t *testing.T) {
	wh := newTestWebhook()
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return wh, nil
		},
	}
	httpClient := &mockHTTPClient{
		doFn: func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 429,
				Body:       io.NopCloser(strings.NewReader("too many requests")),
			}, nil
		},
	}
	worker := NewWebhookWorker(store, httpClient, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte("{}"))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected transient error, got nil")
	}

	// 429 is transient, should not cancel.
	var cancelErr *goriver.JobCancelError
	if errors.As(err, &cancelErr) {
		t.Error("429 should be transient, got JobCancelError")
	}
}

func TestWebhookWorker_ClientError_PermanentFailure(t *testing.T) {
	wh := newTestWebhook()
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return wh, nil
		},
	}
	httpClient := &mockHTTPClient{
		doFn: func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 404,
				Body:       io.NopCloser(strings.NewReader("not found")),
			}, nil
		},
	}
	worker := NewWebhookWorker(store, httpClient, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte("{}"))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// 4xx (not 429) is permanent.
	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError for 404, got %T: %v", err, err)
	}
}

func TestWebhookWorker_NetworkError_Retries(t *testing.T) {
	wh := newTestWebhook()
	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return wh, nil
		},
	}
	httpClient := &mockHTTPClient{
		doFn: func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		},
	}
	worker := NewWebhookWorker(store, httpClient, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte("{}"))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected transient error, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if errors.As(err, &cancelErr) {
		t.Error("network error should be transient, got JobCancelError")
	}
}

func TestWebhookWorker_SuccessResetsFailureCounter(t *testing.T) {
	wh := newTestWebhook()
	wh.ConsecutiveFailures = 5
	now := time.Now()
	wh.LastFailureAt = &now

	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return wh, nil
		},
	}
	httpClient := &mockHTTPClient{}
	worker := NewWebhookWorker(store, httpClient, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte("{}"))
	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have called ResetFailureCount atomically.
	if len(store.resetFailureCalls) != 1 {
		t.Fatalf("expected 1 ResetFailureCount call, got %d", len(store.resetFailureCalls))
	}
	if store.resetFailureCalls[0] != wh.ID {
		t.Errorf("ResetFailureCount called with %v, want %v", store.resetFailureCalls[0], wh.ID)
	}
}

func TestWebhookWorker_AutoDisableAfter10Failures(t *testing.T) {
	wh := newTestWebhook()
	wh.ConsecutiveFailures = 9 // will become 10 with this failure

	store := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return wh, nil
		},
		incrementFailureCountFn: func(_ context.Context, _ uuid.UUID) (int, bool, error) {
			// Simulate: 10th failure triggers auto-disable.
			return 10, false, nil
		},
	}
	httpClient := &mockHTTPClient{
		doFn: func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 500,
				Body:       io.NopCloser(strings.NewReader("error")),
			}, nil
		},
	}
	worker := NewWebhookWorker(store, httpClient, WithSSRFChecker(noopSSRFChecker))

	job := makeWebhookJob(wh.ID, "email.delivered", []byte("{}"))
	_ = worker.Work(context.Background(), job)

	// Should have called IncrementFailureCount atomically.
	if len(store.incrementFailureCalls) == 0 {
		t.Fatal("expected IncrementFailureCount call")
	}
	if store.incrementFailureCalls[0] != wh.ID {
		t.Errorf("IncrementFailureCount called with %v, want %v", store.incrementFailureCalls[0], wh.ID)
	}
}

func TestSignPayload(t *testing.T) {
	secret := "test-secret"
	timestamp := "1234567890"
	payload := []byte(`{"event":"delivered"}`)

	// Compute expected.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	got := signPayload(secret, timestamp, payload)
	if got != expected {
		t.Errorf("signPayload() = %q, want %q", got, expected)
	}
}

func TestSignPayload_DifferentSecrets(t *testing.T) {
	payload := []byte(`{"event":"delivered"}`)
	sig1 := signPayload("secret-a", "1234567890", payload)
	sig2 := signPayload("secret-b", "1234567890", payload)

	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestSignPayload_PrefixFormat(t *testing.T) {
	sig := signPayload("secret", "12345", []byte("test"))
	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("signature should start with 'sha256=', got %q", sig)
	}
}

func TestWebhookWorker_NextRetry_ExponentialBackoff(t *testing.T) {
	worker := &WebhookWorker{}
	tests := []struct {
		attempt     int
		minDuration time.Duration
	}{
		{1, 30 * time.Second},
		{2, 60 * time.Second},
		{3, 120 * time.Second},
		{4, 240 * time.Second},
	}

	for _, tt := range tests {
		job := &goriver.Job[WebhookJobArgs]{
			Args:   WebhookJobArgs{},
			JobRow: &rivertype.JobRow{Attempt: tt.attempt},
		}
		before := time.Now()
		retryAt := worker.NextRetry(job)
		diff := retryAt.Sub(before)
		if diff < tt.minDuration-time.Second || diff > tt.minDuration+time.Second {
			t.Errorf("attempt %d: NextRetry diff = %v, want ~%v", tt.attempt, diff, tt.minDuration)
		}
	}
}

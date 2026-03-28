package river

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	goriver "github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// --- Manual mocks ---

type mockEmailStore struct {
	getByTrackingIDFn func(ctx context.Context, trackingID string) (*domain.Email, error)
	updateStatusFn    func(ctx context.Context, id uuid.UUID, status domain.EmailStatus) error
	updateRetryFn     func(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error
	addEventFn        func(ctx context.Context, event *domain.EmailEvent) error

	updateStatusCalls []updateStatusCall
	addEventCalls     []addEventCall
	updateRetryCalls  []updateRetryCall
}

type updateStatusCall struct {
	ID     uuid.UUID
	Status domain.EmailStatus
}

type addEventCall struct {
	Event *domain.EmailEvent
}

type updateRetryCall struct {
	ID         uuid.UUID
	RetryCount int
}

func (m *mockEmailStore) Create(ctx context.Context, email *domain.Email) error { return nil }
func (m *mockEmailStore) GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error) {
	if m.getByTrackingIDFn != nil {
		return m.getByTrackingIDFn(ctx, trackingID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockEmailStore) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.EmailStatus) error {
	m.updateStatusCalls = append(m.updateStatusCalls, updateStatusCall{ID: id, Status: status})
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}
func (m *mockEmailStore) UpdateRetry(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error {
	m.updateRetryCalls = append(m.updateRetryCalls, updateRetryCall{ID: id, RetryCount: retryCount})
	if m.updateRetryFn != nil {
		return m.updateRetryFn(ctx, id, retryCount, nextRetryAt)
	}
	return nil
}
func (m *mockEmailStore) SetProviderMessageID(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockEmailStore) AddEvent(ctx context.Context, event *domain.EmailEvent) error {
	m.addEventCalls = append(m.addEventCalls, addEventCall{Event: event})
	if m.addEventFn != nil {
		return m.addEventFn(ctx, event)
	}
	return nil
}
func (m *mockEmailStore) GetEvents(ctx context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error) {
	return nil, nil
}
func (m *mockEmailStore) QueryByExternalID(ctx context.Context, wsID uuid.UUID, externalID string, cursor string, limit int) ([]*domain.Email, string, error) {
	return nil, "", nil
}
func (m *mockEmailStore) QueryByRecipient(ctx context.Context, wsID uuid.UUID, email string, cursor string, limit int) ([]*domain.Email, string, error) {
	return nil, "", nil
}
func (m *mockEmailStore) QueryByWorkspace(ctx context.Context, wsID uuid.UUID, filters port.EmailFilters, cursor string, limit int) ([]*domain.Email, string, error) {
	return nil, "", nil
}
func (m *mockEmailStore) QueryByExternalIDGlobal(ctx context.Context, externalID string, cursor string, limit int) ([]*domain.Email, string, error) {
	return nil, "", nil
}

type mockCompiler struct {
	compileFn func(ctx context.Context, mjml string) (string, error)
}

func (m *mockCompiler) Compile(ctx context.Context, mjml string) (string, error) {
	if m.compileFn != nil {
		return m.compileFn(ctx, mjml)
	}
	return "<html>" + mjml + "</html>", nil
}

type mockRenderer struct {
	renderFn func(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error)
}

func (m *mockRenderer) Render(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error) {
	if m.renderFn != nil {
		return m.renderFn(template, injectors, eventVars)
	}
	return template, nil
}

type mockRateLimiter struct {
	tryAcquireFn func(ctx context.Context, adapterID uuid.UUID) (bool, error)
}

func (m *mockRateLimiter) TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error) {
	if m.tryAcquireFn != nil {
		return m.tryAcquireFn(ctx, adapterID)
	}
	return true, nil
}
func (m *mockRateLimiter) SyncBucket(ctx context.Context, adapterID uuid.UUID, maxPerSecond int) error {
	return nil
}

type mockSender struct {
	sendFn func(ctx context.Context, msg *port.OutgoingEmail) (string, error)
	calls  []sendCall
}

type sendCall struct {
	Msg *port.OutgoingEmail
}

func (m *mockSender) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	m.calls = append(m.calls, sendCall{Msg: msg})
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return "provider-msg-123", nil
}
func (m *mockSender) Name() string                          { return "mock" }
func (m *mockSender) HealthCheck(ctx context.Context) error { return nil }

type mockAdapterStore struct {
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Adapter, error)
}

func (m *mockAdapterStore) Create(context.Context, *domain.Adapter) error { return nil }
func (m *mockAdapterStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockAdapterStore) Update(context.Context, *domain.Adapter) error { return nil }
func (m *mockAdapterStore) SoftDelete(context.Context, uuid.UUID) error   { return nil }
func (m *mockAdapterStore) ListInChain(context.Context, []uuid.NullUUID) ([]*domain.Adapter, error) {
	return nil, nil
}
func (m *mockAdapterStore) ListByWorkspace(context.Context, *uuid.UUID, port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	return nil, nil
}

type mockCrypto struct {
	decryptFn func(ciphertext []byte) ([]byte, error)
}

func (m *mockCrypto) Encrypt(_ []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	if m.decryptFn != nil {
		return m.decryptFn(ciphertext)
	}
	return ciphertext, nil
}

// --- Test helpers ---

func newTestEmail() *domain.Email {
	return &domain.Email{
		ID:                uuid.Must(uuid.NewV7()),
		TrackingID:        "trk_test123",
		WorkspaceID:       uuid.Must(uuid.NewV7()),
		TenantID:          uuid.Must(uuid.NewV7()),
		TemplateID:        uuid.Must(uuid.NewV7()),
		TemplateVersionID: uuid.Must(uuid.NewV7()),
		TemplateTypeSlug:  "welcome",
		TemplateRef:       "latam:acme:welcome",
		RecipientEmail:    "user@example.com",
		FromEmail:         "noreply@acme.com",
		FromName:          "Acme",
		SubjectRendered:   "Welcome!",
		AdapterID:         uuid.Must(uuid.NewV7()),
		BodyMJML:          "<mjml><mj-body><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body></mjml>",
		Status:            domain.StatusQueued,
		VariablesSnapshot: map[string]any{"name": "Test"},
		InjectorsSnapshot: map[string]map[string]any{"brand": {"logo": "https://logo.png"}},
		MaxRetries:        3,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
}

func newTestSendWorker(
	emailStore *mockEmailStore,
	compiler *mockCompiler,
	renderer *mockRenderer,
	rateLimiter *mockRateLimiter,
	sender *mockSender,
	opts ...SendWorkerOption,
) *SendWorker {
	var staticSender port.EmailSender
	if sender != nil {
		staticSender = sender
	}
	return NewSendWorker(emailStore, compiler, renderer, rateLimiter, staticSender, opts...)
}

func makeJob(args SendJobArgs, attempt int) *goriver.Job[SendJobArgs] {
	return &goriver.Job[SendJobArgs]{
		Args: args,
		JobRow: &rivertype.JobRow{
			Attempt: attempt,
		},
	}
}

// --- Tests ---

func TestSendWorker_SuccessfulSend(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			if trackingID == email.TrackingID {
				return email, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	sender := &mockSender{}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have status updates: processing, sent.
	if len(emailStore.updateStatusCalls) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(emailStore.updateStatusCalls))
	}
	if emailStore.updateStatusCalls[0].Status != domain.StatusProcessing {
		t.Errorf("first status update = %q, want %q", emailStore.updateStatusCalls[0].Status, domain.StatusProcessing)
	}
	if emailStore.updateStatusCalls[1].Status != domain.StatusSent {
		t.Errorf("second status update = %q, want %q", emailStore.updateStatusCalls[1].Status, domain.StatusSent)
	}

	// Should have called sender.
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sender.calls))
	}
	if sender.calls[0].Msg.To.Address != "user@example.com" {
		t.Errorf("recipient = %q, want %q", sender.calls[0].Msg.To.Address, "user@example.com")
	}
	if sender.calls[0].Msg.Subject != "Welcome!" {
		t.Errorf("subject = %q, want %q", sender.calls[0].Msg.Subject, "Welcome!")
	}

	// Should have events: processing, sent.
	if len(emailStore.addEventCalls) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(emailStore.addEventCalls))
	}
	if emailStore.addEventCalls[0].Event.EventType != domain.StatusProcessing {
		t.Errorf("first event = %q, want %q", emailStore.addEventCalls[0].Event.EventType, domain.StatusProcessing)
	}
	if emailStore.addEventCalls[1].Event.EventType != domain.StatusSent {
		t.Errorf("second event = %q, want %q", emailStore.addEventCalls[1].Event.EventType, domain.StatusSent)
	}
}

func TestSendWorker_EmailNotFound_CancelsJob(t *testing.T) {
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return nil, domain.ErrNotFound
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, &mockSender{})

	job := makeJob(SendJobArgs{TrackingID: "trk_nonexistent"}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should be a JobCancel error (permanent).
	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError, got %T: %v", err, err)
	}
}

func TestSendWorker_AdapterDrivenSender(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			if trackingID == email.TrackingID {
				return email, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	var factoryCalls int
	dynamicSender := &mockSender{}
	adapterStore := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != email.AdapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:              id,
				AdapterType:     domain.AdapterTypeSES,
				ConfigEncrypted: []byte("encrypted"),
			}, nil
		},
	}
	crypto := &mockCrypto{
		decryptFn: func(ciphertext []byte) ([]byte, error) {
			if string(ciphertext) != "encrypted" {
				t.Fatalf("unexpected ciphertext: %q", string(ciphertext))
			}
			return json.RawMessage(`{"region":"us-east-1","access_key_id":"key","secret_access_key":"secret","endpoint_url":"http://localstack:4566"}`), nil
		},
	}

	worker := newTestSendWorker(
		emailStore,
		&mockCompiler{},
		&mockRenderer{},
		&mockRateLimiter{},
		nil,
		WithAdapterRuntime(adapterStore, crypto, func(_ context.Context, adapter *domain.Adapter, decryptedConfig []byte) (port.EmailSender, error) {
			factoryCalls++
			if adapter.AdapterType != domain.AdapterTypeSES {
				t.Fatalf("unexpected adapter type: %s", adapter.AdapterType)
			}
			if string(decryptedConfig) == "" {
				t.Fatal("expected decrypted config")
			}
			return dynamicSender, nil
		}),
	)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if factoryCalls != 1 {
		t.Fatalf("expected 1 sender factory call, got %d", factoryCalls)
	}
	if len(dynamicSender.calls) != 1 {
		t.Fatalf("expected 1 dynamic sender call, got %d", len(dynamicSender.calls))
	}
	if emailStore.updateStatusCalls[len(emailStore.updateStatusCalls)-1].Status != domain.StatusSent {
		t.Fatalf("expected final status sent, got %s", emailStore.updateStatusCalls[len(emailStore.updateStatusCalls)-1].Status)
	}
}

func TestSendWorker_StaticSenderTakesPrecedenceOverAdapterRuntime(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			if trackingID == email.TrackingID {
				return email, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	staticSender := &mockSender{}
	worker := newTestSendWorker(
		emailStore,
		&mockCompiler{},
		&mockRenderer{},
		&mockRateLimiter{},
		staticSender,
		WithAdapterRuntime(&mockAdapterStore{}, &mockCrypto{}, func(_ context.Context, _ *domain.Adapter, _ []byte) (port.EmailSender, error) {
			t.Fatal("adapter runtime should not be used when static sender is configured")
			return nil, nil
		}),
	)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(staticSender.calls) != 1 {
		t.Fatalf("expected static sender to be used once, got %d calls", len(staticSender.calls))
	}
}

func TestSendWorker_RateLimited_Snoozes(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	rateLimiter := &mockRateLimiter{
		tryAcquireFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return false, nil
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, rateLimiter, &mockSender{})

	job := makeJob(SendJobArgs{
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected snooze error, got nil")
	}

	// Should be a JobSnooze error.
	var snoozeErr *goriver.JobSnoozeError
	if !errors.As(err, &snoozeErr) {
		t.Errorf("expected JobSnoozeError, got %T: %v", err, err)
	}

	// Rate limiter check now happens BEFORE status transition (Fix 4),
	// so no status updates should have occurred — email stays "queued".
	if len(emailStore.updateStatusCalls) != 0 {
		t.Errorf("expected 0 status updates when rate limited, got %d", len(emailStore.updateStatusCalls))
	}
}

func TestSendWorker_RenderError_FailsPermanently(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	renderer := &mockRenderer{
		renderFn: func(_ string, _ map[string]map[string]any, _ map[string]any) (string, error) {
			return "", errors.New("render failed: missing variable")
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, renderer, &mockRateLimiter{}, &mockSender{})

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError for render failure, got %T: %v", err, err)
	}

	// Should have marked as failed.
	found := false
	for _, call := range emailStore.updateStatusCalls {
		if call.Status == domain.StatusFailed {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected status update to 'failed'")
	}
}

func TestSendWorker_CompileError_FailsPermanently(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	compiler := &mockCompiler{
		compileFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("compile failed: invalid mjml")
		},
	}
	worker := newTestSendWorker(emailStore, compiler, &mockRenderer{}, &mockRateLimiter{}, &mockSender{})

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError for compile failure, got %T: %v", err, err)
	}
}

func TestSendWorker_TransientSendError_Retries(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	sender := &mockSender{
		sendFn: func(_ context.Context, _ *port.OutgoingEmail) (string, error) {
			return "", errors.New("connection timeout")
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender)

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected transient error, got nil")
	}

	// Should NOT be a JobCancel (it's transient).
	var cancelErr *goriver.JobCancelError
	if errors.As(err, &cancelErr) {
		t.Error("expected transient error but got JobCancelError")
	}

	// Should have recorded retry.
	if len(emailStore.updateRetryCalls) != 1 {
		t.Errorf("expected 1 retry update, got %d", len(emailStore.updateRetryCalls))
	}
}

func TestSendWorker_PermanentSendError_Cancels(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	sender := &mockSender{
		sendFn: func(_ context.Context, _ *port.OutgoingEmail) (string, error) {
			return "", errors.New("address not verified in SES")
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender)

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError for permanent send error, got %T: %v", err, err)
	}
}

func TestSendWorker_CCAndBCC(t *testing.T) {
	email := newTestEmail()
	email.CC = []string{"cc1@example.com", "cc2@example.com"}
	email.BCC = []string{"bcc@example.com"}
	replyTo := "reply@example.com"
	email.ReplyTo = &replyTo

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	sender := &mockSender{}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender)

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := sender.calls[0].Msg
	if len(msg.CC) != 2 {
		t.Errorf("CC count = %d, want 2", len(msg.CC))
	}
	if len(msg.BCC) != 1 {
		t.Errorf("BCC count = %d, want 1", len(msg.BCC))
	}
	if msg.ReplyTo == nil || msg.ReplyTo.Address != "reply@example.com" {
		t.Error("ReplyTo not set correctly")
	}
}

func TestSendWorker_NextRetry_ExponentialBackoff(t *testing.T) {
	worker := &SendWorker{}
	tests := []struct {
		attempt     int
		minDuration time.Duration
	}{
		{1, 60 * time.Second},  // 60s * 2^0 = 60s
		{2, 120 * time.Second}, // 60s * 2^1 = 120s
		{3, 240 * time.Second}, // 60s * 2^2 = 240s
	}

	for _, tt := range tests {
		job := makeJob(SendJobArgs{}, tt.attempt)
		before := time.Now()
		retryAt := worker.NextRetry(job)
		diff := retryAt.Sub(before)
		// Allow 1 second of tolerance.
		if diff < tt.minDuration-time.Second || diff > tt.minDuration+time.Second {
			t.Errorf("attempt %d: NextRetry diff = %v, want ~%v", tt.attempt, diff, tt.minDuration)
		}
	}
}

func TestIsPermanentSendError(t *testing.T) {
	tests := []struct {
		err       string
		permanent bool
	}{
		{"connection timeout", false},
		{"address not verified in SES", true},
		{"rejected by server", true},
		{"invalid sender", true},
		{"blacklisted address", true},
		{"temporary failure", false},
	}
	for _, tt := range tests {
		got := isPermanentSendError(errors.New(tt.err))
		if got != tt.permanent {
			t.Errorf("isPermanentSendError(%q) = %v, want %v", tt.err, got, tt.permanent)
		}
	}
}

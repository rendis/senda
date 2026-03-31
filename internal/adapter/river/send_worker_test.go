package river

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	goriver "github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// --- Manual mocks ---

type mockEmailStore struct {
	getByTrackingIDFn        func(ctx context.Context, trackingID string) (*domain.Email, error)
	updateStatusFn           func(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.EmailStatus) error
	updateRetryFn            func(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error
	addEventFn               func(ctx context.Context, event *domain.EmailEvent) error
	setProviderMessageIDFn   func(ctx context.Context, id uuid.UUID, providerMsgID string) error

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

func (m *mockEmailStore) Create(_ context.Context, _ *domain.Email) error { return nil }
func (m *mockEmailStore) CreateTx(_ context.Context, _ pgx.Tx, _ *domain.Email) error { return nil }
func (m *mockEmailStore) GetByProviderMessageID(_ context.Context, _ string) (*domain.Email, error) {
	return nil, domain.ErrNotFound
}
func (m *mockEmailStore) GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error) {
	if m.getByTrackingIDFn != nil {
		return m.getByTrackingIDFn(ctx, trackingID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockEmailStore) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.EmailStatus) error {
	m.updateStatusCalls = append(m.updateStatusCalls, updateStatusCall{ID: id, Status: newStatus})
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, newStatus, expectedStatus)
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
func (m *mockEmailStore) SetProviderMessageID(ctx context.Context, id uuid.UUID, providerMsgID string) error {
	if m.setProviderMessageIDFn != nil {
		return m.setProviderMessageIDFn(ctx, id, providerMsgID)
	}
	return nil
}
func (m *mockEmailStore) AddEvent(ctx context.Context, event *domain.EmailEvent) error {
	m.addEventCalls = append(m.addEventCalls, addEventCall{Event: event})
	if m.addEventFn != nil {
		return m.addEventFn(ctx, event)
	}
	return nil
}
func (m *mockEmailStore) AddEventTx(_ context.Context, _ pgx.Tx, event *domain.EmailEvent) error {
	m.addEventCalls = append(m.addEventCalls, addEventCall{Event: event})
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
	sendFn                   func(ctx context.Context, msg *port.OutgoingEmail) (string, error)
	isPermanentSendErrorFn   func(err error) bool
	calls                    []sendCall
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
func (m *mockSender) IsPermanentSendError(err error) bool {
	if m.isPermanentSendErrorFn != nil {
		return m.isPermanentSendErrorFn(err)
	}
	return false
}

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
			Attempt:     attempt,
			MaxAttempts: 5, // matches SendJobArgs.InsertOpts MaxAttempts
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
	if emailStore.addEventCalls[0].Event.EventType != domain.EventTypeProcessing {
		t.Errorf("first event = %q, want %q", emailStore.addEventCalls[0].Event.EventType, domain.EventTypeProcessing)
	}
	if emailStore.addEventCalls[1].Event.EventType != domain.EventTypeSent {
		t.Errorf("second event = %q, want %q", emailStore.addEventCalls[1].Event.EventType, domain.EventTypeSent)
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
		isPermanentSendErrorFn: func(_ error) bool { return true },
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
	// With a sender that implements errorClassifier, isPermanentSendError
	// delegates to the sender's classification.
	classifierSender := &mockSender{
		isPermanentSendErrorFn: func(err error) bool {
			msg := err.Error()
			return msg == "address not verified in SES" ||
				msg == "rejected by server" ||
				msg == "invalid sender" ||
				msg == "blacklisted address"
		},
	}
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
		got := isPermanentSendError(classifierSender, errors.New(tt.err))
		if got != tt.permanent {
			t.Errorf("isPermanentSendError(%q) = %v, want %v", tt.err, got, tt.permanent)
		}
	}

	// Without errorClassifier, all errors are treated as transient.
	plainSender := &plainMockSender{}
	for _, tt := range tests {
		got := isPermanentSendError(plainSender, errors.New(tt.err))
		if got {
			t.Errorf("isPermanentSendError(%q) with plain sender = true, want false", tt.err)
		}
	}

	// Nil sender: all errors are transient.
	got := isPermanentSendError(nil, errors.New("any error"))
	if got {
		t.Error("isPermanentSendError with nil sender should return false")
	}
}

func TestSendWorker_SetProviderMessageIDError_ReturnsError(t *testing.T) {
	email := newTestEmail()
	dbErr := errors.New("db: connection lost")
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
		setProviderMessageIDFn: func(_ context.Context, _ uuid.UUID, _ string) error {
			return dbErr
		},
	}
	sender := &mockSender{
		sendFn: func(_ context.Context, _ *port.OutgoingEmail) (string, error) {
			return "provider-msg-abc", nil // non-empty provider ID triggers the DB call
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error when SetProviderMessageID fails, got nil")
	}

	// Must NOT be a JobCancel — should be retryable.
	var cancelErr *goriver.JobCancelError
	if errors.As(err, &cancelErr) {
		t.Errorf("expected retryable error but got JobCancelError: %v", err)
	}

	// Status must NOT be updated to Sent — email should not be marked as sent.
	for _, call := range emailStore.updateStatusCalls {
		if call.Status == domain.StatusSent {
			t.Error("status must not be updated to Sent when SetProviderMessageID fails")
		}
	}
}

// --- C1: Transient error on final attempt must fail permanently ---

// TestSendWorker_TransientSendError_FinalAttempt_FailsPermanently verifies that
// when a transient send error occurs on the last allowed attempt
// (attempt == MaxAttempts), the worker calls failPermanently() so the email
// transitions to Failed instead of being orphaned in Processing forever.
func TestSendWorker_TransientSendError_FinalAttempt_FailsPermanently(t *testing.T) {
	email := newTestEmail()
	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	sender := &mockSender{
		sendFn: func(_ context.Context, _ *port.OutgoingEmail) (string, error) {
			// transient error — IsPermanentSendError returns false (default)
			return "", errors.New("connection timeout")
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender)

	// attempt == MaxAttempts (5): this is the last attempt.
	job := &goriver.Job[SendJobArgs]{
		Args: SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID},
		JobRow: &rivertype.JobRow{
			Attempt:     5,
			MaxAttempts: 5,
		},
	}

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Must be JobCancel, not a raw transient error.
	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError on final attempt, got %T: %v", err, err)
	}

	// Email must have been marked Failed.
	foundFailed := false
	for _, call := range emailStore.updateStatusCalls {
		if call.Status == domain.StatusFailed {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Error("expected status update to 'failed' on final attempt")
	}

	// No retry metadata should have been written.
	if len(emailStore.updateRetryCalls) != 0 {
		t.Errorf("expected 0 retry updates on final attempt, got %d", len(emailStore.updateRetryCalls))
	}
}

// TestSendWorker_TransientSendError_NotFinalAttempt_Retries verifies that a
// transient error on an intermediate attempt still schedules a retry (regression
// guard: C1 fix must not accidentally cancel early retries).
func TestSendWorker_TransientSendError_NotFinalAttempt_Retries(t *testing.T) {
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

	// attempt < MaxAttempts: still has retries left.
	job := &goriver.Job[SendJobArgs]{
		Args: SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID},
		JobRow: &rivertype.JobRow{
			Attempt:     3,
			MaxAttempts: 5,
		},
	}

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected transient error, got nil")
	}

	// Must NOT be a JobCancel.
	var cancelErr *goriver.JobCancelError
	if errors.As(err, &cancelErr) {
		t.Error("transient error on non-final attempt must not produce JobCancelError")
	}

	// Retry metadata must have been written.
	if len(emailStore.updateRetryCalls) != 1 {
		t.Errorf("expected 1 retry update, got %d", len(emailStore.updateRetryCalls))
	}
}

// --- C2: Crash recovery for Processing without ProviderMessageID ---

// TestSendWorker_ProcessingWithoutProviderID_Stale_FailsPermanently verifies
// that when the worker finds an email already in Processing with no
// ProviderMessageID AND the email was last updated more than 10 minutes ago
// (indicating a crash between Send() and SetProviderMessageID), the worker
// calls failPermanently() to unblock it.
func TestSendWorker_ProcessingWithoutProviderID_Stale_FailsPermanently(t *testing.T) {
	email := newTestEmail()
	email.Status = domain.StatusProcessing
	email.ProviderMessageID = nil
	email.UpdatedAt = time.Now().UTC().Add(-15 * time.Minute) // stale

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, &mockSender{})

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Must be JobCancel (permanent failure).
	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError for stale processing email, got %T: %v", err, err)
	}

	// Email must have been marked Failed.
	foundFailed := false
	for _, call := range emailStore.updateStatusCalls {
		if call.Status == domain.StatusFailed {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Error("expected status update to 'failed' for stale processing email")
	}
}

// TestSendWorker_ProcessingWithoutProviderID_Recent_CancelsJob verifies that
// when the email is in Processing with no ProviderMessageID but was updated
// recently (< 10 min), the worker defers to the possible concurrent worker by
// returning JobCancel without marking the email failed.
func TestSendWorker_ProcessingWithoutProviderID_Recent_CancelsJob(t *testing.T) {
	email := newTestEmail()
	email.Status = domain.StatusProcessing
	email.ProviderMessageID = nil
	email.UpdatedAt = time.Now().UTC().Add(-1 * time.Minute) // recent

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, &mockSender{})

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Must be JobCancel.
	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError for recent processing email, got %T: %v", err, err)
	}

	// Email must NOT have been marked Failed — another worker may still own it.
	for _, call := range emailStore.updateStatusCalls {
		if call.Status == domain.StatusFailed {
			t.Error("email should NOT be marked failed when processing is recent (another worker may own it)")
		}
	}
}

// plainMockSender does NOT implement errorClassifier.
type plainMockSender struct{}

func (p *plainMockSender) Send(_ context.Context, _ *port.OutgoingEmail) (string, error) {
	return "", nil
}
func (p *plainMockSender) Name() string                       { return "plain" }
func (p *plainMockSender) HealthCheck(_ context.Context) error { return nil }

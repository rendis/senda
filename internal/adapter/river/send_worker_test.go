package river

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	goriver "github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	smtpadapter "github.com/rendis/senda/internal/adapter/smtp"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// --- Manual mocks ---

type mockEmailStore struct {
	getByTrackingIDFn      func(ctx context.Context, trackingID string) (*domain.Email, error)
	getPayloadFn           func(ctx context.Context, emailID uuid.UUID) (*domain.EmailPayload, error)
	updateStatusFn         func(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.EmailStatus) error
	updateRetryFn          func(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error
	addEventFn             func(ctx context.Context, event *domain.EmailEvent) error
	setProviderMessageIDFn func(ctx context.Context, id uuid.UUID, providerMsgID string) error

	updateStatusCalls []updateStatusCall
	addEventCalls     []addEventCall
	updateRetryCalls  []updateRetryCall
	getPayloadCalls   []uuid.UUID
	lastEmail         *domain.Email
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

func (m *mockEmailStore) Create(_ context.Context, _ *domain.Email) error             { return nil }
func (m *mockEmailStore) CreateTx(_ context.Context, _ pgx.Tx, _ *domain.Email) error { return nil }
func (m *mockEmailStore) GetByProviderMessageID(_ context.Context, _ string) (*domain.Email, error) {
	return nil, domain.ErrNotFound
}
func (m *mockEmailStore) GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error) {
	if m.getByTrackingIDFn != nil {
		email, err := m.getByTrackingIDFn(ctx, trackingID)
		if err == nil {
			m.lastEmail = email
		}
		return email, err
	}
	return nil, domain.ErrNotFound
}
func (m *mockEmailStore) GetPayload(ctx context.Context, emailID uuid.UUID) (*domain.EmailPayload, error) {
	m.getPayloadCalls = append(m.getPayloadCalls, emailID)
	if m.getPayloadFn != nil {
		return m.getPayloadFn(ctx, emailID)
	}
	if m.lastEmail != nil && m.lastEmail.ID == emailID {
		return &domain.EmailPayload{
			EmailID:           m.lastEmail.ID,
			EmailCreatedAt:    m.lastEmail.CreatedAt,
			BodyMJML:          m.lastEmail.BodyMJML,
			VariablesSnapshot: m.lastEmail.VariablesSnapshot,
			InjectorsSnapshot: m.lastEmail.InjectorsSnapshot,
		}, nil
	}
	return nil, domain.ErrNotFound
}
func (m *mockEmailStore) PurgeWorkspaceRuntime(_ context.Context, _ uuid.UUID) error { return nil }
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
func (m *mockEmailStore) DistinctTemplateTypesForRecipient(_ context.Context, _ uuid.UUID, _ string, _ time.Time) ([]port.EmailHistoryType, error) {
	return nil, nil
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
	renderFn            func(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error)
	renderWithSystemFn  func(template string, injectors map[string]map[string]any, eventVars map[string]any, systemVars map[string]string) (string, error)
	lastSystemVars      map[string]string
	renderWithSystemCalls int
}

func (m *mockRenderer) Render(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error) {
	if m.renderFn != nil {
		return m.renderFn(template, injectors, eventVars)
	}
	return template, nil
}

func (m *mockRenderer) RenderWithSystem(template string, injectors map[string]map[string]any, eventVars map[string]any, systemVars map[string]string) (string, error) {
	m.renderWithSystemCalls++
	m.lastSystemVars = systemVars
	if m.renderWithSystemFn != nil {
		return m.renderWithSystemFn(template, injectors, eventVars, systemVars)
	}
	return m.Render(template, injectors, eventVars)
}

type mockRateLimiter struct {
	tryAcquireFn      func(ctx context.Context, adapterID uuid.UUID) (bool, error)
	acquireBurstFn    func(ctx context.Context, adapterID uuid.UUID, requested int) (int, error)
	acquireBurstCalls []acquireBurstCall
}

func (m *mockRateLimiter) TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error) {
	if m.tryAcquireFn != nil {
		return m.tryAcquireFn(ctx, adapterID)
	}
	return true, nil
}

type acquireBurstCall struct {
	AdapterID uuid.UUID
	Requested int
}

func (m *mockRateLimiter) AcquireBurst(ctx context.Context, adapterID uuid.UUID, requested int) (int, error) {
	m.acquireBurstCalls = append(m.acquireBurstCalls, acquireBurstCall{AdapterID: adapterID, Requested: requested})
	if m.acquireBurstFn != nil {
		return m.acquireBurstFn(ctx, adapterID, requested)
	}
	if m.tryAcquireFn != nil {
		allowed, err := m.tryAcquireFn(ctx, adapterID)
		if err != nil {
			return 0, err
		}
		if allowed {
			return 1, nil
		}
		return 0, nil
	}
	return requested, nil
}
func (m *mockRateLimiter) SyncBucket(ctx context.Context, adapterID uuid.UUID, maxPerSecond int) error {
	return nil
}

type mockSender struct {
	sendFn                 func(ctx context.Context, msg *port.OutgoingEmail) (string, error)
	isPermanentSendErrorFn func(err error) bool
	calls                  []sendCall
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

func TestDefaultAdapterSenderFactory_SMTP(t *testing.T) {
	adapter := &domain.Adapter{AdapterType: domain.AdapterTypeSMTP}
	cfg := []byte(`{"host":"localhost","port":1025,"tls_mode":"none"}`)

	sender, err := DefaultAdapterSenderFactory(context.Background(), adapter, cfg)
	if err != nil {
		t.Fatalf("DefaultAdapterSenderFactory() SMTP error = %v", err)
	}
	if sender.Name() != "smtp" {
		t.Fatalf("sender.Name() = %q, want smtp", sender.Name())
	}
}

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

func TestSendWorker_SuccessfulSend_LogsTrackingAndSenderIdentity(t *testing.T) {
	email := newTestEmail()
	senderIdentityID := uuid.Must(uuid.NewV7())
	email.SenderIdentityID = &senderIdentityID

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

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prev)
	})

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "send_worker: email sent") {
		t.Fatalf("expected success log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, email.TrackingID) {
		t.Fatalf("expected tracking_id in log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, email.FromEmail) {
		t.Fatalf("expected from_email in log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, senderIdentityID.String()) {
		t.Fatalf("expected sender_identity_id in log, got %q", logOutput)
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
			return json.RawMessage(`{"region":"us-east-1","access_key_id":"key","secret_access_key":"secret","endpoint_url":"http://aws-sim:4566"}`), nil
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
	if len(emailStore.getPayloadCalls) != 0 {
		t.Errorf("expected 0 payload loads when rate limited, got %d", len(emailStore.getPayloadCalls))
	}
}

func TestSendWorker_LoadsColdPayloadOnlyAfterHotPathChecks(t *testing.T) {
	email := newTestEmail()
	email.BodyMJML = ""
	email.VariablesSnapshot = nil
	email.InjectorsSnapshot = nil

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
		getPayloadFn: func(_ context.Context, emailID uuid.UUID) (*domain.EmailPayload, error) {
			if emailID != email.ID {
				t.Fatalf("expected payload load for %s, got %s", email.ID, emailID)
			}
			return &domain.EmailPayload{
				EmailID:           email.ID,
				BodyMJML:          "<mj-text>Hello {{ name }}</mj-text>",
				VariablesSnapshot: map[string]any{"name": "Ana"},
				InjectorsSnapshot: map[string]map[string]any{"brand": {"name": "Acme"}},
			}, nil
		},
	}
	renderer := &mockRenderer{
		renderFn: func(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error) {
			if got := eventVars["name"]; got != "Ana" {
				t.Fatalf("expected cold payload variables, got %#v", eventVars)
			}
			if got := injectors["brand"]["name"]; got != "Acme" {
				t.Fatalf("expected cold payload injectors, got %#v", injectors)
			}
			return strings.ReplaceAll(template, "{{ name }}", "Ana"), nil
		},
	}
	sender := &mockSender{}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, renderer, &mockRateLimiter{}, sender)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emailStore.getPayloadCalls) != 1 {
		t.Fatalf("expected 1 payload load, got %d", len(emailStore.getPayloadCalls))
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sender.calls))
	}
	if !strings.Contains(sender.calls[0].Msg.BodyHTML, "Hello Ana") {
		t.Fatalf("expected compiled HTML from cold payload, got %q", sender.calls[0].Msg.BodyHTML)
	}
}

func TestSendWorker_ReusesBurstReservationAcrossJobsForSameAdapter(t *testing.T) {
	email1 := newTestEmail()
	email2 := newTestEmail()
	email2.AdapterID = email1.AdapterID

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			switch trackingID {
			case email1.TrackingID:
				return email1, nil
			case email2.TrackingID:
				return email2, nil
			default:
				return nil, domain.ErrNotFound
			}
		},
	}

	rateLimiter := &mockRateLimiter{
		acquireBurstFn: func(_ context.Context, adapterID uuid.UUID, requested int) (int, error) {
			if adapterID != email1.AdapterID {
				t.Fatalf("expected adapter %s, got %s", email1.AdapterID, adapterID)
			}
			if requested < 2 {
				t.Fatalf("expected burst request >= 2, got %d", requested)
			}
			return 2, nil
		},
	}
	sender := &mockSender{}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, rateLimiter, sender)

	job1 := makeJob(SendJobArgs{EmailID: email1.ID, TrackingID: email1.TrackingID, AdapterID: email1.AdapterID}, 1)
	job2 := makeJob(SendJobArgs{EmailID: email2.ID, TrackingID: email2.TrackingID, AdapterID: email2.AdapterID}, 1)

	if err := worker.Work(context.Background(), job1); err != nil {
		t.Fatalf("unexpected error on first job: %v", err)
	}
	if err := worker.Work(context.Background(), job2); err != nil {
		t.Fatalf("unexpected error on second job: %v", err)
	}

	if len(rateLimiter.acquireBurstCalls) != 1 {
		t.Fatalf("expected 1 burst acquisition for same adapter, got %d", len(rateLimiter.acquireBurstCalls))
	}
	if len(sender.calls) != 2 {
		t.Fatalf("expected 2 sends, got %d", len(sender.calls))
	}
}

func TestSendWorker_UsesConfiguredBurstSize(t *testing.T) {
	email := newTestEmail()
	rateLimiter := &mockRateLimiter{
		acquireBurstFn: func(_ context.Context, _ uuid.UUID, requested int) (int, error) {
			return requested, nil
		},
	}
	worker := newTestSendWorker(
		&mockEmailStore{
			getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
				return email, nil
			},
		},
		&mockCompiler{},
		&mockRenderer{},
		rateLimiter,
		&mockSender{},
		WithRateLimitBurstSize(2),
	)

	job := makeJob(SendJobArgs{EmailID: email.ID, TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rateLimiter.acquireBurstCalls) != 1 {
		t.Fatalf("expected 1 burst acquisition, got %d", len(rateLimiter.acquireBurstCalls))
	}
	if rateLimiter.acquireBurstCalls[0].Requested != 2 {
		t.Fatalf("expected configured burst size 2, got %d", rateLimiter.acquireBurstCalls[0].Requested)
	}
}

func TestSendWorker_ReacquiresBurstAfterReservationTTLExpires(t *testing.T) {
	email := newTestEmail()
	base := time.Date(2026, time.April, 11, 12, 0, 0, 0, time.UTC)
	rateLimiter := &mockRateLimiter{
		acquireBurstFn: func(_ context.Context, _ uuid.UUID, requested int) (int, error) {
			return requested, nil
		},
	}
	worker := newTestSendWorker(
		&mockEmailStore{
			getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
				return email, nil
			},
		},
		&mockCompiler{},
		&mockRenderer{},
		rateLimiter,
		&mockSender{},
	)
	worker.now = func() time.Time { return base }
	worker.rateLimitReservationTTL = time.Minute
	worker.rateLimitCleanupInterval = 0

	job := makeJob(SendJobArgs{EmailID: email.ID, TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error on first work: %v", err)
	}
	if len(rateLimiter.acquireBurstCalls) != 1 {
		t.Fatalf("expected first work to reserve once, got %d calls", len(rateLimiter.acquireBurstCalls))
	}

	base = base.Add(2 * time.Minute)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error on second work: %v", err)
	}
	if len(rateLimiter.acquireBurstCalls) != 2 {
		t.Fatalf("expected stale reservation to trigger reacquire, got %d calls", len(rateLimiter.acquireBurstCalls))
	}
}

func TestSendWorker_PrunesStaleRateLimitReservations(t *testing.T) {
	worker := newTestSendWorker(&mockEmailStore{}, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, &mockSender{})
	base := time.Date(2026, time.April, 11, 12, 0, 0, 0, time.UTC)
	staleAdapterID := uuid.Must(uuid.NewV7())
	activeAdapterID := uuid.Must(uuid.NewV7())

	worker.rateLimitReservationTTL = time.Minute
	worker.rateLimits.Store(staleAdapterID, &rateLimitReservation{
		available:   1,
		lastTouched: base.Add(-2 * time.Minute),
	})
	worker.rateLimits.Store(activeAdapterID, &rateLimitReservation{
		available:   1,
		lastTouched: base,
	})

	worker.cleanupExpiredRateLimitReservations(base)

	if _, ok := worker.rateLimits.Load(staleAdapterID); ok {
		t.Fatal("expected stale adapter reservation to be pruned")
	}
	if _, ok := worker.rateLimits.Load(activeAdapterID); !ok {
		t.Fatal("expected active adapter reservation to remain")
	}
}

func TestSendWorker_DoesNotPruneReservationWithOutstandingReference(t *testing.T) {
	worker := newTestSendWorker(&mockEmailStore{}, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, &mockSender{})
	base := time.Date(2026, time.April, 11, 12, 0, 0, 0, time.UTC)
	adapterID := uuid.Must(uuid.NewV7())
	state := &rateLimitReservation{
		available:   1,
		lastTouched: base.Add(-2 * time.Minute),
	}
	state.refs.Add(1)
	worker.rateLimitReservationTTL = time.Minute
	worker.rateLimits.Store(adapterID, state)

	worker.cleanupExpiredRateLimitReservations(base)

	if _, ok := worker.rateLimits.Load(adapterID); !ok {
		t.Fatal("expected reservation with outstanding reference to remain in map")
	}
	if state.deleting.Load() {
		t.Fatal("expected deleting flag to be released after skipped prune")
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

func TestIsPermanentSendError_SMTPStatusCodes(t *testing.T) {
	sender, err := smtpadapter.NewAdapterFromConfig(smtpadapter.Config{
		Host:    "localhost",
		Port:    2525,
		TLSMode: smtpadapter.TLSModeNone,
	})
	if err != nil {
		t.Fatalf("NewAdapterFromConfig() error = %v", err)
	}

	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{name: "5xx", err: &textproto.Error{Code: 550, Msg: "mailbox unavailable"}, permanent: true},
		{name: "4xx", err: &textproto.Error{Code: 450, Msg: "mailbox busy"}, permanent: false},
		{name: "unknown", err: errors.New("connection reset"), permanent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermanentSendError(sender, tt.err)
			if got != tt.permanent {
				t.Fatalf("isPermanentSendError() = %v, want %v", got, tt.permanent)
			}
		})
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

// TestSendWorker_ProcessingWithoutProviderID_Recent_ResumesDelivery verifies
// that when the email is in Processing with no ProviderMessageID but was updated
// recently (< 10 min), the worker resumes the delivery attempt after crash
// recovery instead of cancelling the job permanently.
func TestSendWorker_ProcessingWithoutProviderID_Recent_ResumesDelivery(t *testing.T) {
	email := newTestEmail()
	email.Status = domain.StatusProcessing
	email.ProviderMessageID = nil
	email.UpdatedAt = time.Now().UTC().Add(-1 * time.Minute) // recent

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return email, nil
		},
	}
	sender := &mockSender{
		sendFn: func(_ context.Context, _ *port.OutgoingEmail) (string, error) {
			return "provider-msg-recovered", nil
		},
	}
	worker := newTestSendWorker(emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender)

	job := makeJob(SendJobArgs{TrackingID: email.TrackingID, AdapterID: email.AdapterID}, 1)

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("expected recovered processing email to resume successfully, got %v", err)
	}

	if len(emailStore.updateStatusCalls) == 0 {
		t.Fatal("expected status updates during resumed delivery")
	}
	foundSent := false
	for _, call := range emailStore.updateStatusCalls {
		if call.Status == domain.StatusSent {
			foundSent = true
		}
		if call.Status == domain.StatusFailed {
			t.Fatal("recovered processing email should not be marked failed")
		}
	}
	if !foundSent {
		t.Fatal("expected resumed delivery to mark email as sent")
	}
}

// plainMockSender does NOT implement errorClassifier.
type plainMockSender struct{}

func (p *plainMockSender) Send(_ context.Context, _ *port.OutgoingEmail) (string, error) {
	return "", nil
}
func (p *plainMockSender) Name() string                        { return "plain" }
func (p *plainMockSender) HealthCheck(_ context.Context) error { return nil }

// --- Mocks for unsubscribe header injection ---

type mockTemplateTypeStore struct {
	getTypeBySlugFn func(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error)
}

func (m *mockTemplateTypeStore) CreateType(_ context.Context, _ *domain.TemplateType) error {
	return nil
}
func (m *mockTemplateTypeStore) UpdateType(_ context.Context, _ *domain.TemplateType) error {
	return nil
}
func (m *mockTemplateTypeStore) SoftDeleteType(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockTemplateTypeStore) GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error) {
	if m.getTypeBySlugFn != nil {
		return m.getTypeBySlugFn(ctx, slug, chain)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateTypeStore) FindTypeBySlugInScope(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
	return nil, domain.ErrNotFound
}
func (m *mockTemplateTypeStore) ListTypes(_ context.Context, _ *uuid.UUID, _ port.ListOptions) ([]*domain.TemplateType, string, error) {
	return nil, "", nil
}

type mockWorkspaceStore struct {
	getByIDFn                  func(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	getUnsubscribeSigningKeyFn func(ctx context.Context, workspaceID uuid.UUID) ([]byte, error)
}

func (m *mockWorkspaceStore) Create(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStore) CreateLogicalPair(_ context.Context, _ *domain.Workspace, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &domain.Workspace{ID: id, Name: "Test Workspace"}, nil
}
func (m *mockWorkspaceStore) GetByTenantAndCode(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStore) GetSystemWorkspace(_ context.Context, _ uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStore) ListByTenant(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
	return nil, "", nil
}
func (m *mockWorkspaceStore) UpdateShared(_ context.Context, _ uuid.UUID, _, _, _ string) error {
	return nil
}
func (m *mockWorkspaceStore) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStore) SoftDeleteLogical(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockWorkspaceStore) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockWorkspaceStore) GetUnsubscribeSigningKey(ctx context.Context, workspaceID uuid.UUID) ([]byte, error) {
	if m.getUnsubscribeSigningKeyFn != nil {
		return m.getUnsubscribeSigningKeyFn(ctx, workspaceID)
	}
	return make([]byte, 32), nil // 32-byte zero key
}

// --- Unsubscribe header injection tests ---

func TestSendWorker_BulkTemplate_InjectsListUnsubscribeHeaders(t *testing.T) {
	email := newTestEmail()
	email.TemplateTypeSlug = "newsletter"

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			if trackingID == email.TrackingID {
				return email, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	tts := &mockTemplateTypeStore{
		getTypeBySlugFn: func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			if slug == "newsletter" {
				return &domain.TemplateType{
					ID:     uuid.Must(uuid.NewV7()),
					Slug:   "newsletter",
					Name:   "Newsletter",
					IsBulk: true,
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	signingKey := make([]byte, 32)
	for i := range signingKey {
		signingKey[i] = 0xAB
	}
	wsStore := &mockWorkspaceStore{
		getUnsubscribeSigningKeyFn: func(_ context.Context, _ uuid.UUID) ([]byte, error) {
			return signingKey, nil
		},
	}
	sender := &mockSender{}
	baseURL := "https://senda.example.com"

	worker := newTestSendWorker(
		emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender,
		WithTemplateTypeStore(tts),
		WithWorkspaceLookup(wsStore),
		WithUnsubscribePublicBaseURL(baseURL),
	)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sender.calls))
	}
	hdrs := sender.calls[0].Msg.Headers

	listUnsub, ok := hdrs["List-Unsubscribe"]
	if !ok {
		t.Fatal("expected List-Unsubscribe header, not found")
	}
	if !strings.HasPrefix(listUnsub, "<"+baseURL+"/api/v1/u/") {
		t.Errorf("List-Unsubscribe = %q, want prefix %q", listUnsub, "<"+baseURL+"/api/v1/u/")
	}

	listUnsubPost, ok := hdrs["List-Unsubscribe-Post"]
	if !ok {
		t.Fatal("expected List-Unsubscribe-Post header, not found")
	}
	if listUnsubPost != "List-Unsubscribe=One-Click" {
		t.Errorf("List-Unsubscribe-Post = %q, want %q", listUnsubPost, "List-Unsubscribe=One-Click")
	}

	// Tracking header still present.
	if hdrs["X-Senda-Tracking-ID"] != email.TrackingID {
		t.Errorf("X-Senda-Tracking-ID = %q, want %q", hdrs["X-Senda-Tracking-ID"], email.TrackingID)
	}
}

func TestSendWorker_TransactionalTemplate_NoListUnsubscribeHeaders(t *testing.T) {
	email := newTestEmail()
	email.TemplateTypeSlug = "welcome"

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			if trackingID == email.TrackingID {
				return email, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	tts := &mockTemplateTypeStore{
		getTypeBySlugFn: func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return &domain.TemplateType{
				ID:     uuid.Must(uuid.NewV7()),
				Slug:   slug,
				Name:   "Welcome",
				IsBulk: false, // transactional
			}, nil
		},
	}
	signingKey := make([]byte, 32)
	wsStore := &mockWorkspaceStore{
		getUnsubscribeSigningKeyFn: func(_ context.Context, _ uuid.UUID) ([]byte, error) {
			return signingKey, nil
		},
	}
	sender := &mockSender{}

	worker := newTestSendWorker(
		emailStore, &mockCompiler{}, &mockRenderer{}, &mockRateLimiter{}, sender,
		WithTemplateTypeStore(tts),
		WithWorkspaceLookup(wsStore),
		WithUnsubscribePublicBaseURL("https://senda.example.com"),
	)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sender.calls))
	}
	hdrs := sender.calls[0].Msg.Headers
	if _, ok := hdrs["List-Unsubscribe"]; ok {
		t.Error("unexpected List-Unsubscribe header for transactional template")
	}
	if _, ok := hdrs["List-Unsubscribe-Post"]; ok {
		t.Error("unexpected List-Unsubscribe-Post header for transactional template")
	}
}

func TestSendWorker_BulkTemplate_PassesSystemVarsToRenderer(t *testing.T) {
	email := newTestEmail()
	email.TemplateTypeSlug = "promo"

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			if trackingID == email.TrackingID {
				return email, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	tts := &mockTemplateTypeStore{
		getTypeBySlugFn: func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return &domain.TemplateType{
				ID:     uuid.Must(uuid.NewV7()),
				Slug:   slug,
				Name:   "Promo",
				IsBulk: true,
			}, nil
		},
	}
	signingKey := make([]byte, 32)
	for i := range signingKey {
		signingKey[i] = 0xCC
	}
	wsStore := &mockWorkspaceStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			return &domain.Workspace{ID: id, Name: "Acme Workspace"}, nil
		},
		getUnsubscribeSigningKeyFn: func(_ context.Context, _ uuid.UUID) ([]byte, error) {
			return signingKey, nil
		},
	}
	renderer := &mockRenderer{}
	baseURL := "https://senda.example.com"

	worker := newTestSendWorker(
		emailStore, &mockCompiler{}, renderer, &mockRateLimiter{}, &mockSender{},
		WithTemplateTypeStore(tts),
		WithWorkspaceLookup(wsStore),
		WithUnsubscribePublicBaseURL(baseURL),
	)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if renderer.renderWithSystemCalls == 0 {
		t.Fatal("expected RenderWithSystem to be called")
	}
	sv := renderer.lastSystemVars
	if sv == nil {
		t.Fatal("expected non-nil systemVars passed to RenderWithSystem")
	}
	if _, ok := sv["unsubscribe_url"]; !ok {
		t.Error("expected systemVars to contain unsubscribe_url")
	}
	if _, ok := sv["preferences_url"]; !ok {
		t.Error("expected systemVars to contain preferences_url")
	}
	if sv["workspace_name"] != "Acme Workspace" {
		t.Errorf("workspace_name = %q, want %q", sv["workspace_name"], "Acme Workspace")
	}
	if !strings.HasPrefix(sv["unsubscribe_url"], baseURL+"/u/") {
		t.Errorf("unsubscribe_url = %q, want prefix %q", sv["unsubscribe_url"], baseURL+"/u/")
	}
}

// TestSendWorker_BulkPath_TemplateTypeLookupTransientError_SkipsHeaders verifies
// that a non-NotFound error from GetTypeBySlug is logged as a warning but does
// NOT abort the send. The email is delivered without List-Unsubscribe headers
// and the renderer receives nil systemVars.
func TestSendWorker_BulkPath_TemplateTypeLookupTransientError_SkipsHeaders(t *testing.T) {
	email := newTestEmail()
	email.TemplateTypeSlug = "newsletter"

	emailStore := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			if trackingID == email.TrackingID {
				return email, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	// Simulate a transient (non-NotFound) error from the template type store.
	transientErr := errors.New("db: connection reset by peer")
	tts := &mockTemplateTypeStore{
		getTypeBySlugFn: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return nil, transientErr
		},
	}
	wsStore := &mockWorkspaceStore{}
	sender := &mockSender{}
	renderer := &mockRenderer{}

	// Capture slog output to assert the warning is emitted.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	worker := newTestSendWorker(
		emailStore, &mockCompiler{}, renderer, &mockRateLimiter{}, sender,
		WithTemplateTypeStore(tts),
		WithWorkspaceLookup(wsStore),
		WithUnsubscribePublicBaseURL("https://senda.example.com"),
	)

	job := makeJob(SendJobArgs{
		EmailID:    email.ID,
		TrackingID: email.TrackingID,
		AdapterID:  email.AdapterID,
	}, 1)

	// Send must succeed — a lookup error is non-fatal.
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("expected successful send despite template type lookup error, got: %v", err)
	}

	// The warning must have been logged.
	logOutput := buf.String()
	if !strings.Contains(logOutput, "failed to resolve template type for unsubscribe headers") {
		t.Errorf("expected warning log for template type lookup error, got: %q", logOutput)
	}

	// The email must have been delivered.
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sender.calls))
	}

	// No List-Unsubscribe headers must be present.
	hdrs := sender.calls[0].Msg.Headers
	if _, ok := hdrs["List-Unsubscribe"]; ok {
		t.Error("expected no List-Unsubscribe header when template type lookup failed")
	}
	if _, ok := hdrs["List-Unsubscribe-Post"]; ok {
		t.Error("expected no List-Unsubscribe-Post header when template type lookup failed")
	}

	// Renderer must have been called with nil systemVars (no unsubscribe context).
	if renderer.lastSystemVars != nil {
		t.Errorf("expected nil systemVars passed to renderer, got %v", renderer.lastSystemVars)
	}
}

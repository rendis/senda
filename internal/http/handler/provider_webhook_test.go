package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/service"
)

// --- Provider webhook mocks ---

type mockSNSVerifier struct {
	verifyFn func(message []byte) error
}

func (m *mockSNSVerifier) Verify(message []byte) error {
	if m.verifyFn != nil {
		return m.verifyFn(message)
	}
	return nil
}

type mockSubscriptionConfirmer struct {
	confirmFn func(ctx context.Context, subscribeURL string) error
	calls     []string
}

func (m *mockSubscriptionConfirmer) ConfirmSubscription(ctx context.Context, subscribeURL string) error {
	m.calls = append(m.calls, subscribeURL)
	if m.confirmFn != nil {
		return m.confirmFn(ctx, subscribeURL)
	}
	return nil
}

// mockEmailLookupPW implements service.EmailLookup for provider webhook tests.
type mockEmailLookupPW struct {
	getByProviderMessageIDFn func(ctx context.Context, providerMessageID string) (*domain.Email, error)
}

func (m *mockEmailLookupPW) GetByProviderMessageID(ctx context.Context, providerMessageID string) (*domain.Email, error) {
	if m.getByProviderMessageIDFn != nil {
		return m.getByProviderMessageIDFn(ctx, providerMessageID)
	}
	return nil, domain.ErrNotFound
}

// mockEmailUpdaterPW implements service.EmailStatusUpdater for provider webhook tests.
type mockEmailUpdaterPW struct {
	statuses []domain.EmailStatus
	events   []*domain.EmailEvent
}

func (m *mockEmailUpdaterPW) UpdateStatus(_ context.Context, _ uuid.UUID, newStatus, _ domain.EmailStatus) error {
	m.statuses = append(m.statuses, newStatus)
	return nil
}

func (m *mockEmailUpdaterPW) AddEvent(_ context.Context, event *domain.EmailEvent) error {
	m.events = append(m.events, event)
	return nil
}

// mockSuppressionWriterPW implements service.SuppressionWriter for provider webhook tests.
type mockSuppressionWriterPW struct {
	globals    []*domain.SuppressionGlobal
	workspaces []*domain.SuppressionWorkspace
}

func (m *mockSuppressionWriterPW) AddGlobal(_ context.Context, entry *domain.SuppressionGlobal) error {
	m.globals = append(m.globals, entry)
	return nil
}

func (m *mockSuppressionWriterPW) AddWorkspace(_ context.Context, entry *domain.SuppressionWorkspace) error {
	m.workspaces = append(m.workspaces, entry)
	return nil
}

// mockWebhookDispatcherPW implements service.WebhookDispatcher for provider webhook tests.
type mockWebhookDispatcherPW struct {
	calls []struct {
		EventType string
		WsID      uuid.UUID
	}
}

func (m *mockWebhookDispatcherPW) Dispatch(_ context.Context, wsID uuid.UUID, eventType string, _ any) error {
	m.calls = append(m.calls, struct {
		EventType string
		WsID      uuid.UUID
	}{eventType, wsID})
	return nil
}

// --- Provider webhook test fixture ---

type providerWebhookFixture struct {
	emailID     uuid.UUID
	workspaceID uuid.UUID

	verifier   *mockSNSVerifier
	confirmer  *mockSubscriptionConfirmer
	lookup     *mockEmailLookupPW
	updater    *mockEmailUpdaterPW
	suppressor *mockSuppressionWriterPW
	dispatcher *mockWebhookDispatcherPW
}

func newProviderWebhookFixture() *providerWebhookFixture {
	emailID := uuid.Must(uuid.NewV7())
	wsID := uuid.Must(uuid.NewV7())

	f := &providerWebhookFixture{
		emailID:     emailID,
		workspaceID: wsID,
		verifier:    &mockSNSVerifier{},
		confirmer:   &mockSubscriptionConfirmer{},
		lookup:      &mockEmailLookupPW{},
		updater:     &mockEmailUpdaterPW{},
		suppressor:  &mockSuppressionWriterPW{},
		dispatcher:  &mockWebhookDispatcherPW{},
	}

	f.lookup.getByProviderMessageIDFn = func(_ context.Context, msgID string) (*domain.Email, error) {
		if msgID == "ses-msg-id-001" {
			return &domain.Email{
				ID:             emailID,
				WorkspaceID:    wsID,
				TrackingID:     "trk_test",
				RecipientEmail: "alice@user.com",
				Status:         domain.StatusSent,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	return f
}

func (f *providerWebhookFixture) buildHandler() *handler.SESWebhookHandler {
	processor := service.NewEventProcessor(
		f.lookup,
		f.updater,
		f.suppressor,
		f.dispatcher,
		nil,
	)
	return handler.NewSESWebhookHandler(processor, f.verifier, f.confirmer, nil)
}

func (f *providerWebhookFixture) snsDeliveryNotification() []byte {
	sesEvent := map[string]any{
		"notificationType": "Delivery",
		"mail":             map[string]any{"messageId": "ses-msg-id-001"},
		"delivery":         map[string]any{"timestamp": "2026-02-17T10:00:00.000Z"},
	}
	sesJSON, _ := json.Marshal(sesEvent)

	msg := map[string]any{
		"Type":             "Notification",
		"MessageId":        "sns-msg-001",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          string(sesJSON),
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)
	return body
}

func (f *providerWebhookFixture) snsBounceNotification(bounceType string) []byte {
	sesEvent := map[string]any{
		"notificationType": "Bounce",
		"mail":             map[string]any{"messageId": "ses-msg-id-001"},
		"bounce": map[string]any{
			"bounceType":        bounceType,
			"bouncedRecipients": []map[string]any{{"emailAddress": "alice@user.com"}},
			"timestamp":         "2026-02-17T10:00:00.000Z",
		},
	}
	sesJSON, _ := json.Marshal(sesEvent)

	msg := map[string]any{
		"Type":             "Notification",
		"MessageId":        "sns-msg-002",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          string(sesJSON),
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)
	return body
}

func (f *providerWebhookFixture) snsComplaintNotification() []byte {
	sesEvent := map[string]any{
		"notificationType": "Complaint",
		"mail":             map[string]any{"messageId": "ses-msg-id-001"},
		"complaint": map[string]any{
			"complaintFeedbackType": "abuse",
			"complainedRecipients":  []map[string]any{{"emailAddress": "alice@user.com"}},
			"feedbackId":            "fb-123",
			"timestamp":             "2026-02-17T10:00:00.000Z",
		},
	}
	sesJSON, _ := json.Marshal(sesEvent)

	msg := map[string]any{
		"Type":             "Notification",
		"MessageId":        "sns-msg-003",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          string(sesJSON),
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)
	return body
}

func (f *providerWebhookFixture) snsSubscriptionConfirmation() []byte {
	msg := map[string]any{
		"Type":             "SubscriptionConfirmation",
		"MessageId":        "sns-sub-001",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          "You have chosen to subscribe to the topic...",
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"Token":            "abc123token",
		"SubscribeURL":     "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&TopicArn=arn:aws:sns:us-east-1:123456789012:SES-Events&Token=abc123token",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)
	return body
}

// --- Tests ---

func TestSESWebhook_Delivery_HappyPath(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	body := f.snsDeliveryNotification()
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)
	c.Request().Header.Set("Content-Type", "application/json")

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Verify status was updated to delivered
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusDelivered {
		t.Fatalf("expected status 'delivered', got %v", f.updater.statuses)
	}

	// Verify webhook was dispatched
	if len(f.dispatcher.calls) != 1 || f.dispatcher.calls[0].EventType != "email.delivered" {
		t.Fatalf("expected webhook for 'email.delivered', got %v", f.dispatcher.calls)
	}
}

func TestSESWebhook_PermanentBounce(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	body := f.snsBounceNotification("Permanent")
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Status updated to bounced
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusBounced {
		t.Fatalf("expected status 'bounced', got %v", f.updater.statuses)
	}

	// Global suppression added (hard bounce)
	if len(f.suppressor.globals) != 1 {
		t.Fatalf("expected 1 global suppression for hard bounce, got %d", len(f.suppressor.globals))
	}
	if f.suppressor.globals[0].Email != "alice@user.com" {
		t.Fatalf("expected suppression for 'alice@user.com', got %q", f.suppressor.globals[0].Email)
	}
	if f.suppressor.globals[0].Reason != domain.SuppressionHardBounce {
		t.Fatalf("expected reason 'hard_bounce', got %q", f.suppressor.globals[0].Reason)
	}
}

func TestSESWebhook_TransientBounce_NoSuppression(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	body := f.snsBounceNotification("Transient")
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Status updated to bounced
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusBounced {
		t.Fatalf("expected status 'bounced', got %v", f.updater.statuses)
	}

	// No suppression for soft bounce
	if len(f.suppressor.globals) != 0 {
		t.Fatalf("expected no global suppressions for transient bounce, got %d", len(f.suppressor.globals))
	}
}

func TestSESWebhook_Complaint(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	body := f.snsComplaintNotification()
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Status updated to complained
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusComplained {
		t.Fatalf("expected status 'complained', got %v", f.updater.statuses)
	}

	// Workspace suppression added
	if len(f.suppressor.workspaces) != 1 {
		t.Fatalf("expected 1 workspace suppression for complaint, got %d", len(f.suppressor.workspaces))
	}
	if f.suppressor.workspaces[0].Email != "alice@user.com" {
		t.Fatalf("expected suppression for 'alice@user.com', got %q", f.suppressor.workspaces[0].Email)
	}
	if f.suppressor.workspaces[0].Reason != domain.SuppressionComplaint {
		t.Fatalf("expected reason 'complaint', got %q", f.suppressor.workspaces[0].Reason)
	}
}

func TestSESWebhook_SubscriptionConfirmation(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	body := f.snsSubscriptionConfirmation()
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Confirm subscription URL was fetched
	if len(f.confirmer.calls) != 1 {
		t.Fatalf("expected 1 subscription confirmation call, got %d", len(f.confirmer.calls))
	}
	expectedURL := "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&TopicArn=arn:aws:sns:us-east-1:123456789012:SES-Events&Token=abc123token"
	if f.confirmer.calls[0] != expectedURL {
		t.Fatalf("expected SubscribeURL %q, got %q", expectedURL, f.confirmer.calls[0])
	}

	// No email processing
	if len(f.updater.statuses) != 0 {
		t.Fatalf("expected no status updates for subscription confirmation, got %d", len(f.updater.statuses))
	}
}

func TestSESWebhook_SkipSignatureVerificationOption(t *testing.T) {
	f := newProviderWebhookFixture()
	f.verifier.verifyFn = func(_ []byte) error {
		return errors.New("bad signature")
	}

	processor := service.NewEventProcessor(
		f.lookup,
		f.updater,
		f.suppressor,
		f.dispatcher,
		nil,
	)
	h := handler.NewSESWebhookHandler(
		processor,
		f.verifier,
		f.confirmer,
		nil,
		handler.WithSkipSignatureVerification(true),
	)

	body := f.snsDeliveryNotification()
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)
	c.Request().Header.Set("Content-Type", "application/json")

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(f.updater.statuses) == 0 || f.updater.statuses[0] != domain.StatusDelivered {
		t.Fatalf("expected delivery to be processed despite verifier failure, got statuses=%v", f.updater.statuses)
	}
}

func TestSESWebhook_SignatureVerificationFails(t *testing.T) {
	f := newProviderWebhookFixture()
	f.verifier.verifyFn = func(_ []byte) error {
		return errors.New("signature invalid")
	}
	h := f.buildHandler()

	body := f.snsDeliveryNotification()
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for invalid signature, got %d", rec.Code)
	}

	// No processing
	if len(f.updater.statuses) != 0 {
		t.Fatalf("expected no status updates after failed verification, got %d", len(f.updater.statuses))
	}
}

func TestSESWebhook_MalformedJSON(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader([]byte(`{broken`))),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for malformed JSON, got %d", rec.Code)
	}
}

func TestSESWebhook_UnknownNotificationType(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	sesEvent := map[string]any{
		"notificationType": "Unknown",
		"mail":             map[string]any{"messageId": "ses-msg-id-001"},
	}
	sesJSON, _ := json.Marshal(sesEvent)

	msg := map[string]any{
		"Type":             "Notification",
		"MessageId":        "sns-msg-999",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          string(sesJSON),
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Returns 200 (doesn't block SNS retries)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for unknown notification type, got %d", rec.Code)
	}

	// No processing
	if len(f.updater.statuses) != 0 {
		t.Fatalf("expected no status updates for unknown notification type, got %d", len(f.updater.statuses))
	}
}

func TestSESWebhook_UnknownSNSMessageType(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	msg := map[string]any{
		"Type":             "UnsubscribeConfirmation",
		"MessageId":        "sns-unsub-001",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          "unsubscribe",
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for unknown SNS type, got %d", rec.Code)
	}
}

func TestSESWebhook_EmailNotFound_Returns200(t *testing.T) {
	f := newProviderWebhookFixture()
	f.lookup.getByProviderMessageIDFn = func(_ context.Context, _ string) (*domain.Email, error) {
		return nil, domain.ErrNotFound
	}
	h := f.buildHandler()

	body := f.snsDeliveryNotification()
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must return 200 even on processing errors — to prevent SNS retries
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 even when email not found, got %d", rec.Code)
	}
}

func TestSESWebhook_MalformedSESMessage_Returns200(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	msg := map[string]any{
		"Type":             "Notification",
		"MessageId":        "sns-msg-bad",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          "{not valid json",
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Return 200 to prevent SNS retries on permanently malformed messages
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for malformed SES message, got %d", rec.Code)
	}
}

func TestSESWebhook_NilVerifier(t *testing.T) {
	f := newProviderWebhookFixture()
	processor := service.NewEventProcessor(
		f.lookup,
		f.updater,
		f.suppressor,
		f.dispatcher,
		nil,
	)
	h := handler.NewSESWebhookHandler(processor, nil, f.confirmer, nil) // nil verifier

	body := f.snsDeliveryNotification()
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with nil verifier, got %d", rec.Code)
	}

	// Event should still be processed
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusDelivered {
		t.Fatalf("expected delivery processed with nil verifier, got %v", f.updater.statuses)
	}
}

func TestSESWebhook_SubscriptionConfirmation_MissingURL(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	msg := map[string]any{
		"Type":             "SubscriptionConfirmation",
		"MessageId":        "sns-sub-bad",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          "subscribe",
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
		// No SubscribeURL
	}
	body, _ := json.Marshal(msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing SubscribeURL, got %d", rec.Code)
	}
}

func TestSESWebhook_InvalidTopicArn_Returns400(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	msg := map[string]any{
		"Type":             "Notification",
		"MessageId":        "sns-msg-bad-arn",
		"TopicArn":         "not-an-arn",
		"Message":          `{"notificationType":"Delivery","mail":{"messageId":"ses-msg-id-001"},"delivery":{"timestamp":"2026-02-17T10:00:00.000Z"}}`,
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid TopicArn, got %d", rec.Code)
	}

	// No email processing should occur
	if len(f.updater.statuses) != 0 {
		t.Fatalf("expected no status updates for invalid TopicArn, got %d", len(f.updater.statuses))
	}
}

func TestSESWebhook_SubscriptionConfirmation_SSRF_InvalidScheme(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	msg := map[string]any{
		"Type":             "SubscriptionConfirmation",
		"MessageId":        "sns-sub-ssrf",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          "subscribe",
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"Token":            "abc123token",
		"SubscribeURL":     "http://internal-server.local/admin",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for SSRF attempt (invalid scheme), got %d", rec.Code)
	}

	// No subscription confirmation should have been made
	if len(f.confirmer.calls) != 0 {
		t.Fatalf("expected no confirmation calls for SSRF attempt, got %d", len(f.confirmer.calls))
	}
}

func TestSESWebhook_SubscriptionConfirmation_SSRF_InvalidHost(t *testing.T) {
	f := newProviderWebhookFixture()
	h := f.buildHandler()

	msg := map[string]any{
		"Type":             "SubscriptionConfirmation",
		"MessageId":        "sns-sub-ssrf2",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          "subscribe",
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"Token":            "abc123token",
		"SubscribeURL":     "https://evil.example.com/callback",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := json.Marshal(msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for SSRF attempt (invalid host), got %d", rec.Code)
	}

	if len(f.confirmer.calls) != 0 {
		t.Fatalf("expected no confirmation calls for SSRF attempt, got %d", len(f.confirmer.calls))
	}
}

// Suppress unused import warning for time package
var _ = time.Now

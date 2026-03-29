package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/service"
)

// --- EventProcessor mocks ---

type mockEmailLookup struct {
	getByProviderMessageIDFn func(ctx context.Context, providerMessageID string) (*domain.Email, error)
}

func (m *mockEmailLookup) GetByProviderMessageID(ctx context.Context, providerMessageID string) (*domain.Email, error) {
	if m.getByProviderMessageIDFn != nil {
		return m.getByProviderMessageIDFn(ctx, providerMessageID)
	}
	return nil, domain.ErrNotFound
}

type mockEmailStatusUpdater struct {
	updateStatusFn func(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.EmailStatus) error
	addEventFn     func(ctx context.Context, event *domain.EmailEvent) error
	statuses       []domain.EmailStatus
	events         []*domain.EmailEvent
}

func (m *mockEmailStatusUpdater) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.EmailStatus) error {
	m.statuses = append(m.statuses, newStatus)
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, newStatus, expectedStatus)
	}
	return nil
}

func (m *mockEmailStatusUpdater) AddEvent(ctx context.Context, event *domain.EmailEvent) error {
	m.events = append(m.events, event)
	if m.addEventFn != nil {
		return m.addEventFn(ctx, event)
	}
	return nil
}

type mockSuppressionWriter struct {
	addGlobalFn    func(ctx context.Context, entry *domain.SuppressionGlobal) error
	addWorkspaceFn func(ctx context.Context, entry *domain.SuppressionWorkspace) error
	globals        []*domain.SuppressionGlobal
	workspaces     []*domain.SuppressionWorkspace
}

func (m *mockSuppressionWriter) AddGlobal(ctx context.Context, entry *domain.SuppressionGlobal) error {
	m.globals = append(m.globals, entry)
	if m.addGlobalFn != nil {
		return m.addGlobalFn(ctx, entry)
	}
	return nil
}

func (m *mockSuppressionWriter) AddWorkspace(ctx context.Context, entry *domain.SuppressionWorkspace) error {
	m.workspaces = append(m.workspaces, entry)
	if m.addWorkspaceFn != nil {
		return m.addWorkspaceFn(ctx, entry)
	}
	return nil
}

type mockWebhookDispatcher struct {
	dispatchFn func(ctx context.Context, workspaceID uuid.UUID, eventType string, payload any) error
	calls      []webhookDispatchCall
}

type webhookDispatchCall struct {
	WorkspaceID uuid.UUID
	EventType   string
	Payload     any
}

func (m *mockWebhookDispatcher) Dispatch(ctx context.Context, workspaceID uuid.UUID, eventType string, payload any) error {
	m.calls = append(m.calls, webhookDispatchCall{
		WorkspaceID: workspaceID,
		EventType:   eventType,
		Payload:     payload,
	})
	if m.dispatchFn != nil {
		return m.dispatchFn(ctx, workspaceID, eventType, payload)
	}
	return nil
}

// --- EventProcessor test fixture ---

type eventProcessorFixture struct {
	emailID     uuid.UUID
	workspaceID uuid.UUID

	lookup     *mockEmailLookup
	updater    *mockEmailStatusUpdater
	suppressor *mockSuppressionWriter
	dispatcher *mockWebhookDispatcher
}

func newEventProcessorFixture() *eventProcessorFixture {
	emailID := uuid.Must(uuid.NewV7())
	wsID := uuid.Must(uuid.NewV7())

	f := &eventProcessorFixture{
		emailID:     emailID,
		workspaceID: wsID,
		lookup:      &mockEmailLookup{},
		updater:     &mockEmailStatusUpdater{},
		suppressor:  &mockSuppressionWriter{},
		dispatcher:  &mockWebhookDispatcher{},
	}

	f.lookup.getByProviderMessageIDFn = func(_ context.Context, providerMessageID string) (*domain.Email, error) {
		if providerMessageID == "ses-msg-123" {
			return &domain.Email{
				ID:             emailID,
				WorkspaceID:    wsID,
				TrackingID:     "trk_abc123",
				RecipientEmail: "alice@user.com",
				Status:         domain.StatusSent,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	return f
}

func (f *eventProcessorFixture) buildProcessor() *service.EventProcessor {
	return service.NewEventProcessor(
		f.lookup,
		f.updater,
		f.suppressor,
		f.dispatcher,
		nil, // use default logger
	)
}

func (f *eventProcessorFixture) deliveryEvent() *domain.ProviderEvent {
	return &domain.ProviderEvent{
		Type:              domain.EventDelivered,
		ProviderMessageID: "ses-msg-123",
		Timestamp:         time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC),
		RawPayload:        json.RawMessage(`{}`),
	}
}

func (f *eventProcessorFixture) hardBounceEvent() *domain.ProviderEvent {
	return &domain.ProviderEvent{
		Type:              domain.EventBounced,
		ProviderMessageID: "ses-msg-123",
		Timestamp:         time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC),
		RawPayload:        json.RawMessage(`{}`),
		BounceDetail: &domain.BounceDetail{
			BounceType:     "hard",
			DiagnosticCode: "smtp;550 5.1.1",
			Recipients:     []string{"alice@user.com"},
		},
	}
}

func (f *eventProcessorFixture) softBounceEvent() *domain.ProviderEvent {
	return &domain.ProviderEvent{
		Type:              domain.EventBounced,
		ProviderMessageID: "ses-msg-123",
		Timestamp:         time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC),
		RawPayload:        json.RawMessage(`{}`),
		BounceDetail: &domain.BounceDetail{
			BounceType:     "soft",
			DiagnosticCode: "smtp;452 4.2.2",
			Recipients:     []string{"alice@user.com"},
		},
	}
}

func (f *eventProcessorFixture) complaintEvent() *domain.ProviderEvent {
	return &domain.ProviderEvent{
		Type:              domain.EventComplained,
		ProviderMessageID: "ses-msg-123",
		Timestamp:         time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC),
		RawPayload:        json.RawMessage(`{}`),
		ComplaintDetail: &domain.ComplaintDetail{
			ComplaintType: "abuse",
			FeedbackID:    "feedback-456",
			Recipients:    []string{"alice@user.com"},
		},
	}
}

func (f *eventProcessorFixture) openEvent() *domain.ProviderEvent {
	return &domain.ProviderEvent{
		Type:              domain.EventOpened,
		ProviderMessageID: "ses-msg-123",
		Timestamp:         time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC),
		RawPayload:        json.RawMessage(`{}`),
	}
}

// --- Tests ---

func TestEventProcessor_Delivery_HappyPath(t *testing.T) {
	f := newEventProcessorFixture()
	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.deliveryEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status updated to delivered
	if len(f.updater.statuses) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(f.updater.statuses))
	}
	if f.updater.statuses[0] != domain.StatusDelivered {
		t.Fatalf("expected status 'delivered', got %q", f.updater.statuses[0])
	}

	// Email event added
	if len(f.updater.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(f.updater.events))
	}
	if f.updater.events[0].EventType != domain.EventTypeDelivered {
		t.Fatalf("expected event type 'delivered', got %q", f.updater.events[0].EventType)
	}
	if f.updater.events[0].EmailID != f.emailID {
		t.Fatalf("expected email ID %s, got %s", f.emailID, f.updater.events[0].EmailID)
	}

	// No suppression entries
	if len(f.suppressor.globals) != 0 {
		t.Fatalf("expected no global suppressions, got %d", len(f.suppressor.globals))
	}
	if len(f.suppressor.workspaces) != 0 {
		t.Fatalf("expected no workspace suppressions, got %d", len(f.suppressor.workspaces))
	}

	// Webhook dispatched
	if len(f.dispatcher.calls) != 1 {
		t.Fatalf("expected 1 webhook dispatch, got %d", len(f.dispatcher.calls))
	}
	if f.dispatcher.calls[0].EventType != "email.delivered" {
		t.Fatalf("expected event type 'email.delivered', got %q", f.dispatcher.calls[0].EventType)
	}
	if f.dispatcher.calls[0].WorkspaceID != f.workspaceID {
		t.Fatalf("expected workspace ID %s, got %s", f.workspaceID, f.dispatcher.calls[0].WorkspaceID)
	}
}

func TestEventProcessor_HardBounce_SuppressionAndStatus(t *testing.T) {
	f := newEventProcessorFixture()
	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.hardBounceEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status updated to bounced
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusBounced {
		t.Fatalf("expected status 'bounced', got %v", f.updater.statuses)
	}

	// Global suppression added for hard bounce
	if len(f.suppressor.globals) != 1 {
		t.Fatalf("expected 1 global suppression, got %d", len(f.suppressor.globals))
	}
	entry := f.suppressor.globals[0]
	if entry.Email != "alice@user.com" {
		t.Fatalf("expected suppressed email 'alice@user.com', got %q", entry.Email)
	}
	if entry.Reason != domain.SuppressionHardBounce {
		t.Fatalf("expected reason 'hard_bounce', got %q", entry.Reason)
	}
	if entry.SourceEmailID == nil || *entry.SourceEmailID != f.emailID {
		t.Fatalf("expected source email ID %s", f.emailID)
	}

	// No workspace suppression
	if len(f.suppressor.workspaces) != 0 {
		t.Fatalf("expected no workspace suppressions, got %d", len(f.suppressor.workspaces))
	}

	// Webhook dispatched
	if len(f.dispatcher.calls) != 1 || f.dispatcher.calls[0].EventType != "email.bounced" {
		t.Fatalf("expected webhook dispatch for 'email.bounced'")
	}
}

func TestEventProcessor_SoftBounce_NoSuppression(t *testing.T) {
	f := newEventProcessorFixture()
	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.softBounceEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status updated to bounced
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusBounced {
		t.Fatalf("expected status 'bounced', got %v", f.updater.statuses)
	}

	// No suppression for soft bounce
	if len(f.suppressor.globals) != 0 {
		t.Fatalf("expected no global suppressions for soft bounce, got %d", len(f.suppressor.globals))
	}
	if len(f.suppressor.workspaces) != 0 {
		t.Fatalf("expected no workspace suppressions for soft bounce, got %d", len(f.suppressor.workspaces))
	}
}

func TestEventProcessor_Complaint_WorkspaceSuppression(t *testing.T) {
	f := newEventProcessorFixture()
	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.complaintEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status updated to complained
	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusComplained {
		t.Fatalf("expected status 'complained', got %v", f.updater.statuses)
	}

	// Workspace suppression added (not global)
	if len(f.suppressor.globals) != 0 {
		t.Fatalf("expected no global suppressions for complaint, got %d", len(f.suppressor.globals))
	}
	if len(f.suppressor.workspaces) != 1 {
		t.Fatalf("expected 1 workspace suppression, got %d", len(f.suppressor.workspaces))
	}
	ws := f.suppressor.workspaces[0]
	if ws.Email != "alice@user.com" {
		t.Fatalf("expected suppressed email 'alice@user.com', got %q", ws.Email)
	}
	if ws.Reason != domain.SuppressionComplaint {
		t.Fatalf("expected reason 'complaint', got %q", ws.Reason)
	}
	if ws.WorkspaceID != f.workspaceID {
		t.Fatalf("expected workspace ID %s, got %s", f.workspaceID, ws.WorkspaceID)
	}
	if ws.SourceEmailID == nil || *ws.SourceEmailID != f.emailID {
		t.Fatalf("expected source email ID %s", f.emailID)
	}

	// Event metadata has complaint details
	if len(f.updater.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(f.updater.events))
	}
	meta := f.updater.events[0].Metadata
	if meta["complaint_type"] != "abuse" {
		t.Fatalf("expected complaint_type 'abuse', got %v", meta["complaint_type"])
	}
	if meta["feedback_id"] != "feedback-456" {
		t.Fatalf("expected feedback_id 'feedback-456', got %v", meta["feedback_id"])
	}
}

func TestEventProcessor_Open_NoSuppression(t *testing.T) {
	f := newEventProcessorFixture()
	// An email must be delivered before it can be opened.
	f.lookup.getByProviderMessageIDFn = func(_ context.Context, providerMessageID string) (*domain.Email, error) {
		if providerMessageID == "ses-msg-123" {
			return &domain.Email{
				ID:             f.emailID,
				WorkspaceID:    f.workspaceID,
				TrackingID:     "trk_abc123",
				RecipientEmail: "alice@user.com",
				Status:         domain.StatusDelivered,
			}, nil
		}
		return nil, domain.ErrNotFound
	}
	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.openEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.updater.statuses) != 1 || f.updater.statuses[0] != domain.StatusOpened {
		t.Fatalf("expected status 'opened', got %v", f.updater.statuses)
	}

	// No suppression for open events
	if len(f.suppressor.globals) != 0 || len(f.suppressor.workspaces) != 0 {
		t.Fatal("expected no suppressions for open event")
	}
}

func TestEventProcessor_EmailNotFound(t *testing.T) {
	f := newEventProcessorFixture()
	proc := f.buildProcessor()

	event := f.deliveryEvent()
	event.ProviderMessageID = "unknown-msg-id"

	err := proc.Process(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for unknown provider message ID")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// No status updates or events created
	if len(f.updater.statuses) != 0 {
		t.Fatalf("expected no status updates, got %d", len(f.updater.statuses))
	}
}

func TestEventProcessor_UpdateStatusError_PropagatesError(t *testing.T) {
	f := newEventProcessorFixture()
	f.updater.updateStatusFn = func(_ context.Context, _ uuid.UUID, _, _ domain.EmailStatus) error {
		return errors.New("db update failed")
	}

	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.deliveryEvent())
	if err == nil {
		t.Fatal("expected error when UpdateStatus fails")
	}
	if err.Error() != "db update failed" {
		t.Fatalf("expected 'db update failed', got %q", err.Error())
	}
}

func TestEventProcessor_AddEventError_PropagatesError(t *testing.T) {
	f := newEventProcessorFixture()
	f.updater.addEventFn = func(_ context.Context, _ *domain.EmailEvent) error {
		return errors.New("event insert failed")
	}

	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.deliveryEvent())
	if err == nil {
		t.Fatal("expected error when AddEvent fails")
	}
	if err.Error() != "event insert failed" {
		t.Fatalf("expected 'event insert failed', got %q", err.Error())
	}
}

func TestEventProcessor_SuppressionError_DoesNotBlock(t *testing.T) {
	f := newEventProcessorFixture()
	f.suppressor.addGlobalFn = func(_ context.Context, _ *domain.SuppressionGlobal) error {
		return errors.New("suppression insert failed")
	}

	proc := f.buildProcessor()

	// Hard bounce with suppression failure should still succeed
	err := proc.Process(context.Background(), f.hardBounceEvent())
	if err != nil {
		t.Fatalf("suppression error should not block processing, got: %v", err)
	}

	// Status and event should still be updated
	if len(f.updater.statuses) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(f.updater.statuses))
	}
}

func TestEventProcessor_WebhookError_DoesNotBlock(t *testing.T) {
	f := newEventProcessorFixture()
	f.dispatcher.dispatchFn = func(_ context.Context, _ uuid.UUID, _ string, _ any) error {
		return errors.New("webhook dispatch failed")
	}

	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.deliveryEvent())
	if err != nil {
		t.Fatalf("webhook error should not block processing, got: %v", err)
	}

	// Status and event should still be updated
	if len(f.updater.statuses) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(f.updater.statuses))
	}
}

func TestEventProcessor_NilWebhookDispatcher(t *testing.T) {
	f := newEventProcessorFixture()
	proc := service.NewEventProcessor(
		f.lookup,
		f.updater,
		f.suppressor,
		nil, // no webhook dispatcher
		nil,
	)

	err := proc.Process(context.Background(), f.deliveryEvent())
	if err != nil {
		t.Fatalf("unexpected error with nil webhook dispatcher: %v", err)
	}

	// Status and event should still be updated
	if len(f.updater.statuses) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(f.updater.statuses))
	}
}

func TestEventProcessor_BounceMetadata(t *testing.T) {
	f := newEventProcessorFixture()
	proc := f.buildProcessor()

	err := proc.Process(context.Background(), f.hardBounceEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.updater.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(f.updater.events))
	}
	meta := f.updater.events[0].Metadata
	if meta["bounce_type"] != "hard" {
		t.Fatalf("expected bounce_type 'hard', got %v", meta["bounce_type"])
	}
	if meta["diagnostic_code"] != "smtp;550 5.1.1" {
		t.Fatalf("expected diagnostic_code 'smtp;550 5.1.1', got %v", meta["diagnostic_code"])
	}
	if meta["source"] != "provider_webhook" {
		t.Fatalf("expected source 'provider_webhook', got %v", meta["source"])
	}
}

package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// --- Mock WebhookStore ---

type mockWebhookStore struct {
	createFn              func(ctx context.Context, wh *domain.Webhook) error
	getByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	updateFn              func(ctx context.Context, wh *domain.Webhook) error
	deleteFn              func(ctx context.Context, id uuid.UUID) error
	listByWorkspaceFn     func(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Webhook], error)
	getActiveByWorkspaceFn func(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Webhook, error)
}

func (m *mockWebhookStore) Create(ctx context.Context, wh *domain.Webhook) error {
	if m.createFn != nil {
		return m.createFn(ctx, wh)
	}
	return nil
}
func (m *mockWebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockWebhookStore) Update(ctx context.Context, wh *domain.Webhook) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, wh)
	}
	return nil
}
func (m *mockWebhookStore) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}
func (m *mockWebhookStore) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Webhook], error) {
	if m.listByWorkspaceFn != nil {
		return m.listByWorkspaceFn(ctx, workspaceID, opts)
	}
	return &port.PageResult[domain.Webhook]{Items: []*domain.Webhook{}}, nil
}
func (m *mockWebhookStore) GetActiveByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Webhook, error) {
	if m.getActiveByWorkspaceFn != nil {
		return m.getActiveByWorkspaceFn(ctx, workspaceID)
	}
	return nil, nil
}

// --- Mock JobQueue for webhook tests ---

type mockJobQueue struct {
	enqueueSendFn    func(ctx context.Context, job *port.SendJob) error
	enqueueWebhookFn func(ctx context.Context, job *port.WebhookJob) error
}

func (m *mockJobQueue) EnqueueSend(ctx context.Context, job *port.SendJob) error {
	if m.enqueueSendFn != nil {
		return m.enqueueSendFn(ctx, job)
	}
	return nil
}
func (m *mockJobQueue) EnqueueWebhook(ctx context.Context, job *port.WebhookJob) error {
	if m.enqueueWebhookFn != nil {
		return m.enqueueWebhookFn(ctx, job)
	}
	return nil
}

// --- Tests ---

func TestWebhookService_Dispatch_EnqueuesMatchingWebhooks(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	wh1ID := uuid.Must(uuid.NewV7())
	wh2ID := uuid.Must(uuid.NewV7())

	now := time.Now().UTC()
	store := &mockWebhookStore{
		getActiveByWorkspaceFn: func(_ context.Context, wID uuid.UUID) ([]*domain.Webhook, error) {
			if wID != wsID {
				t.Fatalf("expected workspace ID %s, got %s", wsID, wID)
			}
			return []*domain.Webhook{
				{ID: wh1ID, WorkspaceID: wsID, URL: "https://example.com/hook1", Events: []string{"email.sent", "email.delivered"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
				{ID: wh2ID, WorkspaceID: wsID, URL: "https://example.com/hook2", Events: []string{"email.bounced"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}

	var enqueuedJobs []*port.WebhookJob
	queue := &mockJobQueue{
		enqueueWebhookFn: func(_ context.Context, job *port.WebhookJob) error {
			enqueuedJobs = append(enqueuedJobs, job)
			return nil
		},
	}

	svc := service.NewWebhookService(store, queue)

	payload := map[string]string{"email_id": "123"}
	err := svc.Dispatch(context.Background(), wsID, "email.sent", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only wh1 subscribes to email.sent.
	if len(enqueuedJobs) != 1 {
		t.Fatalf("expected 1 enqueued job, got %d", len(enqueuedJobs))
	}
	if enqueuedJobs[0].WebhookID != wh1ID {
		t.Fatalf("expected webhook ID %s, got %s", wh1ID, enqueuedJobs[0].WebhookID)
	}
	if enqueuedJobs[0].EventType != "email.sent" {
		t.Fatalf("expected event type 'email.sent', got %q", enqueuedJobs[0].EventType)
	}

	// Verify payload is JSON-encoded.
	var decoded map[string]string
	if err := json.Unmarshal(enqueuedJobs[0].Payload, &decoded); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if decoded["email_id"] != "123" {
		t.Fatalf("expected payload email_id '123', got %q", decoded["email_id"])
	}
}

func TestWebhookService_Dispatch_WildcardMatchesAll(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	whID := uuid.Must(uuid.NewV7())

	now := time.Now().UTC()
	store := &mockWebhookStore{
		getActiveByWorkspaceFn: func(_ context.Context, _ uuid.UUID) ([]*domain.Webhook, error) {
			return []*domain.Webhook{
				{ID: whID, WorkspaceID: wsID, URL: "https://example.com/hook", Events: []string{"*"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}

	var enqueuedCount int
	queue := &mockJobQueue{
		enqueueWebhookFn: func(_ context.Context, _ *port.WebhookJob) error {
			enqueuedCount++
			return nil
		},
	}

	svc := service.NewWebhookService(store, queue)

	err := svc.Dispatch(context.Background(), wsID, "any.event.type", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueuedCount != 1 {
		t.Fatalf("expected 1 enqueued job for wildcard, got %d", enqueuedCount)
	}
}

func TestWebhookService_Dispatch_NoActiveWebhooks(t *testing.T) {
	store := &mockWebhookStore{
		getActiveByWorkspaceFn: func(_ context.Context, _ uuid.UUID) ([]*domain.Webhook, error) {
			return nil, nil
		},
	}

	var enqueuedCount int
	queue := &mockJobQueue{
		enqueueWebhookFn: func(_ context.Context, _ *port.WebhookJob) error {
			enqueuedCount++
			return nil
		},
	}

	svc := service.NewWebhookService(store, queue)

	err := svc.Dispatch(context.Background(), uuid.Must(uuid.NewV7()), "email.sent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueuedCount != 0 {
		t.Fatalf("expected 0 enqueued jobs, got %d", enqueuedCount)
	}
}

func TestWebhookService_Dispatch_NoMatchingEvents(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	store := &mockWebhookStore{
		getActiveByWorkspaceFn: func(_ context.Context, _ uuid.UUID) ([]*domain.Webhook, error) {
			return []*domain.Webhook{
				{ID: uuid.Must(uuid.NewV7()), WorkspaceID: wsID, URL: "https://example.com/hook", Events: []string{"email.bounced"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}

	var enqueuedCount int
	queue := &mockJobQueue{
		enqueueWebhookFn: func(_ context.Context, _ *port.WebhookJob) error {
			enqueuedCount++
			return nil
		},
	}

	svc := service.NewWebhookService(store, queue)

	err := svc.Dispatch(context.Background(), wsID, "email.sent", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueuedCount != 0 {
		t.Fatalf("expected 0 enqueued jobs, got %d", enqueuedCount)
	}
}

func TestWebhookService_Dispatch_StoreError_LogsAndReturnsNil(t *testing.T) {
	store := &mockWebhookStore{
		getActiveByWorkspaceFn: func(_ context.Context, _ uuid.UUID) ([]*domain.Webhook, error) {
			return nil, errors.New("database down")
		},
	}

	svc := service.NewWebhookService(store, &mockJobQueue{})

	// Dispatch is fire-and-forget: store errors are logged, not propagated.
	err := svc.Dispatch(context.Background(), uuid.Must(uuid.NewV7()), "email.sent", nil)
	if err != nil {
		t.Fatalf("expected nil (fire-and-forget), got %v", err)
	}
}

func TestWebhookService_Dispatch_EnqueueError_LogsAndContinues(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	store := &mockWebhookStore{
		getActiveByWorkspaceFn: func(_ context.Context, _ uuid.UUID) ([]*domain.Webhook, error) {
			return []*domain.Webhook{
				{ID: uuid.Must(uuid.NewV7()), WorkspaceID: wsID, URL: "https://example.com/hook", Events: []string{"*"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}

	queue := &mockJobQueue{
		enqueueWebhookFn: func(_ context.Context, _ *port.WebhookJob) error {
			return errors.New("queue full")
		},
	}

	svc := service.NewWebhookService(store, queue)

	// Dispatch is fire-and-forget: enqueue errors are logged, not propagated.
	err := svc.Dispatch(context.Background(), wsID, "email.sent", map[string]string{})
	if err != nil {
		t.Fatalf("expected nil (fire-and-forget), got %v", err)
	}
}

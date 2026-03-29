package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// --- Mock WebhookStore ---

type mockWebhookStore struct {
	createFn               func(ctx context.Context, wh *domain.Webhook) error
	getByIDFn              func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	updateFn               func(ctx context.Context, wh *domain.Webhook) error
	deleteFn               func(ctx context.Context, id uuid.UUID) error
	listByWorkspaceFn      func(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Webhook], error)
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
func (m *mockWebhookStore) IncrementFailureCount(_ context.Context, _ uuid.UUID) (int, bool, error) {
	return 0, true, nil
}
func (m *mockWebhookStore) ResetFailureCount(_ context.Context, _ uuid.UUID) error {
	return nil
}

// --- Mock JobQueue ---

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
func (m *mockJobQueue) EnqueueSendTx(_ context.Context, _ pgx.Tx, _ *port.SendJob) error {
	return nil
}
func (m *mockJobQueue) EnqueueWebhook(ctx context.Context, job *port.WebhookJob) error {
	if m.enqueueWebhookFn != nil {
		return m.enqueueWebhookFn(ctx, job)
	}
	return nil
}

// --- Helpers ---

func setupWebhookTest(whs port.WebhookStore, q port.JobQueue, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.WebhookHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	svc := service.NewWebhookService(whs, q)
	h := handler.NewWebhookHandler(whs, svc, ts, ws)

	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/webhooks", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/webhooks", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id", h.Update)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id", h.Delete)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id/test", h.Test)

	return e, h
}

// --- Tests ---

func TestWebhookHandler_Create_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var created *domain.Webhook
	whs := &mockWebhookStore{
		createFn: func(_ context.Context, wh *domain.Webhook) error {
			created = wh
			return nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	body := `{"url":"https://example.com/webhook","events":["email.sent","email.delivered"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/webhooks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected webhook to be created")
	}
	if created.WorkspaceID != ws.ID {
		t.Fatalf("expected workspace ID %s, got %s", ws.ID, created.WorkspaceID)
	}
	if created.URL != "https://example.com/webhook" {
		t.Fatalf("expected URL 'https://example.com/webhook', got %q", created.URL)
	}
	if len(created.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(created.Events))
	}
	if !created.IsActive {
		t.Fatal("expected is_active=true")
	}
	// Secret should be 64 hex chars (32 bytes).
	if len(created.Secret) != 64 {
		t.Fatalf("expected secret length 64, got %d", len(created.Secret))
	}

	// Verify response contains the secret.
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	secret, ok := resp["secret"].(string)
	if !ok || secret == "" {
		t.Fatal("response must contain 'secret' field")
	}
	if secret != created.Secret {
		t.Fatalf("expected response secret to match created secret")
	}
}

func TestWebhookHandler_Create_SecretReturned(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	whs := &mockWebhookStore{}
	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	body := `{"url":"https://example.com/hook","events":["*"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/webhooks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["secret"]; !ok {
		t.Fatal("create response must contain 'secret' field")
	}
}

func TestWebhookHandler_Create_MissingURL(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupWebhookTest(&mockWebhookStore{}, &mockJobQueue{}, ts, wsStore)

	body := `{"events":["email.sent"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/webhooks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Create_InvalidURL(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupWebhookTest(&mockWebhookStore{}, &mockJobQueue{}, ts, wsStore)

	body := `{"url":"http://insecure.com/hook","events":["email.sent"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/webhooks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Create_MissingEvents(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupWebhookTest(&mockWebhookStore{}, &mockJobQueue{}, ts, wsStore)

	body := `{"url":"https://example.com/hook"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/webhooks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_List_SecretHidden(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	whs := &mockWebhookStore{
		listByWorkspaceFn: func(_ context.Context, wID uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.Webhook], error) {
			if wID != ws.ID {
				t.Fatalf("expected workspace ID %s, got %s", ws.ID, wID)
			}
			return &port.PageResult[domain.Webhook]{
				Items: []*domain.Webhook{
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: ws.ID, URL: "https://example.com/hook", Secret: "supersecret", Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
				},
				HasMore: false,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/webhooks", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw["items"], &items); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if _, ok := items[0]["secret"]; ok {
		t.Fatal("list response must NOT contain 'secret' field")
	}
}

func TestWebhookHandler_Get_SecretHidden(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/hook", Secret: "supersecret",
				Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["secret"]; ok {
		t.Fatal("get response must NOT contain 'secret' field")
	}
}

func TestWebhookHandler_Get_WrongWorkspace(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	otherWsID := uuid.Must(uuid.NewV7())
	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: otherWsID, URL: "https://example.com/hook",
				Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Get_InvalidID(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupWebhookTest(&mockWebhookStore{}, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Update_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var updatedWH *domain.Webhook
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/old",
				Secret: "secret", Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		updateFn: func(_ context.Context, wh *domain.Webhook) error {
			updatedWH = wh
			return nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	body := `{"url":"https://example.com/new","events":["email.sent","email.bounced"],"is_active":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updatedWH == nil {
		t.Fatal("expected webhook to be updated")
	}
	if updatedWH.URL != "https://example.com/new" {
		t.Fatalf("expected URL 'https://example.com/new', got %q", updatedWH.URL)
	}
	if len(updatedWH.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(updatedWH.Events))
	}
	if updatedWH.IsActive {
		t.Fatal("expected is_active=false")
	}

	// Update response should not contain secret.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["secret"]; ok {
		t.Fatal("update response must NOT contain 'secret' field")
	}
}

func TestWebhookHandler_Update_EmptyURL(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/hook",
				Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	body := `{"url":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Update_InvalidURL(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/hook",
				Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	body := `{"url":"http://insecure.com"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Update_EmptyEvents(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/hook",
				Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	body := `{"events":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Delete_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var deletedID uuid.UUID
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/hook",
				Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		deleteFn: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if deletedID != webhookID {
		t.Fatalf("expected deleted ID %s, got %s", webhookID, deletedID)
	}
}

func TestWebhookHandler_Delete_WrongWorkspace(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	otherWsID := uuid.Must(uuid.NewV7())
	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: otherWsID, URL: "https://example.com/hook",
				Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Delete_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	webhookID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_Test_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/hook",
				Events: []string{"*"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		getActiveByWorkspaceFn: func(_ context.Context, _ uuid.UUID) ([]*domain.Webhook, error) {
			return []*domain.Webhook{
				{ID: webhookID, WorkspaceID: ws.ID, URL: "https://example.com/hook", Events: []string{"*"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}

	var enqueuedJob *port.WebhookJob
	queue := &mockJobQueue{
		enqueueWebhookFn: func(_ context.Context, job *port.WebhookJob) error {
			enqueuedJob = job
			return nil
		},
	}

	e, _ := setupWebhookTest(whs, queue, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String()+"/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if enqueuedJob == nil {
		t.Fatal("expected webhook job to be enqueued")
	}
	if enqueuedJob.EventType != "webhook.test" {
		t.Fatalf("expected event type 'webhook.test', got %q", enqueuedJob.EventType)
	}
}

func TestWebhookHandler_Test_WrongWorkspace(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	otherWsID := uuid.Must(uuid.NewV7())
	webhookID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	whs := &mockWebhookStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID: webhookID, WorkspaceID: otherWsID, URL: "https://example.com/hook",
				Events: []string{"*"}, IsActive: true, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/webhooks/"+webhookID.String()+"/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandler_List_Pagination(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	failed := now.Add(-1 * time.Hour)
	disabled := now.Add(-30 * time.Minute)

	whs := &mockWebhookStore{
		listByWorkspaceFn: func(_ context.Context, _ uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.Webhook], error) {
			return &port.PageResult[domain.Webhook]{
				Items: []*domain.Webhook{
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: ws.ID, URL: "https://example.com/hook1", Events: []string{"email.sent"}, IsActive: true, CreatedAt: now, UpdatedAt: now},
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: ws.ID, URL: "https://example.com/hook2", Events: []string{"*"}, IsActive: false, ConsecutiveFailures: 5, LastFailureAt: &failed, DisabledAt: &disabled, CreatedAt: now, UpdatedAt: now},
				},
				HasMore:    true,
				NextCursor: "cursor456",
			}, nil
		},
	}

	e, _ := setupWebhookTest(whs, &mockJobQueue{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/webhooks?limit=2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"next_cursor"`
		HasMore    bool             `json:"has_more"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
	if resp.NextCursor != "cursor456" {
		t.Fatalf("expected next_cursor 'cursor456', got %q", resp.NextCursor)
	}

	// Verify failure fields on second item.
	item2 := resp.Items[1]
	if item2["consecutive_failures"] != float64(5) {
		t.Fatalf("expected consecutive_failures=5, got %v", item2["consecutive_failures"])
	}
	if item2["last_failure_at"] == nil {
		t.Fatal("expected last_failure_at to be present")
	}
	if item2["disabled_at"] == nil {
		t.Fatal("expected disabled_at to be present")
	}
}

func TestWebhookHandler_Create_InvalidWorkspace(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}
	wsStore := &mockWorkspaceStore{}

	e, _ := setupWebhookTest(&mockWebhookStore{}, &mockJobQueue{}, ts, wsStore)

	body := `{"url":"https://example.com/hook","events":["email.sent"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/nonexistent/workspaces/default/webhooks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

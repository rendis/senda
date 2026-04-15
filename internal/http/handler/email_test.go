package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
)

// --- Mock EmailStore ---

type mockEmailStore struct {
	createFn                  func(ctx context.Context, email *domain.Email) error
	getByTrackingIDFn         func(ctx context.Context, trackingID string) (*domain.Email, error)
	updateStatusFn            func(ctx context.Context, id uuid.UUID, status domain.EmailStatus) error
	updateRetryFn             func(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error
	addEventFn                func(ctx context.Context, event *domain.EmailEvent) error
	getEventsFn               func(ctx context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error)
	queryByExternalIDFn       func(ctx context.Context, wsID uuid.UUID, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
	queryByRecipientFn        func(ctx context.Context, wsID uuid.UUID, email string, cursor string, limit int) ([]*domain.Email, string, error)
	queryByWorkspaceFn        func(ctx context.Context, wsID uuid.UUID, filters port.EmailFilters, cursor string, limit int) ([]*domain.Email, string, error)
	queryByExternalIDGlobalFn func(ctx context.Context, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
	purgeWorkspaceFn          func(ctx context.Context, workspaceID uuid.UUID) error
}

func (m *mockEmailStore) Create(ctx context.Context, email *domain.Email) error {
	if m.createFn != nil {
		return m.createFn(ctx, email)
	}
	return nil
}
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
func (m *mockEmailStore) GetPayload(_ context.Context, _ uuid.UUID) (*domain.EmailPayload, error) {
	return nil, nil
}
func (m *mockEmailStore) PurgeWorkspaceRuntime(ctx context.Context, workspaceID uuid.UUID) error {
	if m.purgeWorkspaceFn != nil {
		return m.purgeWorkspaceFn(ctx, workspaceID)
	}
	return nil
}
func (m *mockEmailStore) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus, _ domain.EmailStatus) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, newStatus)
	}
	return nil
}
func (m *mockEmailStore) UpdateRetry(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error {
	if m.updateRetryFn != nil {
		return m.updateRetryFn(ctx, id, retryCount, nextRetryAt)
	}
	return nil
}
func (m *mockEmailStore) SetProviderMessageID(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockEmailStore) AddEvent(ctx context.Context, event *domain.EmailEvent) error {
	if m.addEventFn != nil {
		return m.addEventFn(ctx, event)
	}
	return nil
}
func (m *mockEmailStore) AddEventTx(_ context.Context, _ pgx.Tx, _ *domain.EmailEvent) error {
	return nil
}
func (m *mockEmailStore) GetEvents(ctx context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error) {
	if m.getEventsFn != nil {
		return m.getEventsFn(ctx, emailID)
	}
	return nil, nil
}
func (m *mockEmailStore) QueryByExternalID(ctx context.Context, wsID uuid.UUID, externalID string, cursor string, limit int) ([]*domain.Email, string, error) {
	if m.queryByExternalIDFn != nil {
		return m.queryByExternalIDFn(ctx, wsID, externalID, cursor, limit)
	}
	return nil, "", nil
}
func (m *mockEmailStore) QueryByRecipient(ctx context.Context, wsID uuid.UUID, email string, cursor string, limit int) ([]*domain.Email, string, error) {
	if m.queryByRecipientFn != nil {
		return m.queryByRecipientFn(ctx, wsID, email, cursor, limit)
	}
	return nil, "", nil
}
func (m *mockEmailStore) QueryByWorkspace(ctx context.Context, wsID uuid.UUID, filters port.EmailFilters, cursor string, limit int) ([]*domain.Email, string, error) {
	if m.queryByWorkspaceFn != nil {
		return m.queryByWorkspaceFn(ctx, wsID, filters, cursor, limit)
	}
	return nil, "", nil
}
func (m *mockEmailStore) QueryByExternalIDGlobal(ctx context.Context, externalID string, cursor string, limit int) ([]*domain.Email, string, error) {
	if m.queryByExternalIDGlobalFn != nil {
		return m.queryByExternalIDGlobalFn(ctx, externalID, cursor, limit)
	}
	return nil, "", nil
}

// --- Helpers ---

func setupEmailTest(es port.EmailStore, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.EmailHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewEmailHandler(es, ts, ws)

	base := "/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code"
	e.GET(base+"/emails", h.List)
	e.GET(base+"/emails/:tracking_id", h.GetByTrackingID)
	e.GET(base+"/emails/:tracking_id/events", h.GetEvents)

	return e, h
}

// --- Tests ---

func TestEmailHandler_List_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	emailID := uuid.Must(uuid.NewV7())
	es := &mockEmailStore{
		queryByWorkspaceFn: func(_ context.Context, wsID uuid.UUID, _ port.EmailFilters, _ string, _ int) ([]*domain.Email, string, error) {
			return []*domain.Email{
				{
					ID: emailID, TrackingID: "trk_abc123", WorkspaceID: ws.ID, TenantID: ws.TenantID,
					TemplateTypeSlug: "welcome", TemplateRef: "acme:default:welcome",
					RecipientEmail: "user@example.com", FromEmail: "no-reply@acme.com",
					FromName: "Acme", SubjectRendered: "Welcome!", Status: domain.StatusSent,
					CreatedAt: now, UpdatedAt: now,
				},
			}, "", nil
		},
	}

	e, _ := setupEmailTest(es, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/emails", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(resp["items"], &items); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["tracking_id"] != "trk_abc123" {
		t.Fatalf("expected tracking_id trk_abc123, got %v", items[0]["tracking_id"])
	}
}

func TestEmailHandler_List_WithFilters(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var capturedFilters port.EmailFilters
	es := &mockEmailStore{
		queryByWorkspaceFn: func(_ context.Context, _ uuid.UUID, f port.EmailFilters, _ string, _ int) ([]*domain.Email, string, error) {
			capturedFilters = f
			return nil, "", nil
		},
	}

	e, _ := setupEmailTest(es, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/emails?status=sent&template_type=welcome&since=2024-01-01T00:00:00Z&until=2024-12-31T23:59:59Z", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if capturedFilters.Status == nil || *capturedFilters.Status != domain.StatusSent {
		t.Fatal("expected status filter 'sent'")
	}
	if capturedFilters.TemplateTypeSlug == nil || *capturedFilters.TemplateTypeSlug != "welcome" {
		t.Fatal("expected template_type filter 'welcome'")
	}
	if capturedFilters.Since == nil {
		t.Fatal("expected since filter")
	}
	if capturedFilters.Until == nil {
		t.Fatal("expected until filter")
	}
}

func TestEmailHandler_GetByTrackingID_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	emailID := uuid.Must(uuid.NewV7())
	es := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			return &domain.Email{
				ID: emailID, TrackingID: trackingID, WorkspaceID: ws.ID, TenantID: ws.TenantID,
				TemplateTypeSlug: "welcome", TemplateRef: "acme:default:welcome",
				RecipientEmail: "user@example.com", FromEmail: "no-reply@acme.com",
				FromName: "Acme", SubjectRendered: "Welcome!", Status: domain.StatusSent,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		getEventsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.EmailEvent, error) {
			return []*domain.EmailEvent{
				{ID: uuid.Must(uuid.NewV7()), EmailID: emailID, EventType: domain.EventTypeSent, OccurredAt: now, CreatedAt: now},
			}, nil
		},
	}

	e, _ := setupEmailTest(es, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/emails/trk_abc123", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := resp["events"]; !ok {
		t.Fatal("expected events field in detail response")
	}
}

func TestEmailHandler_GetByTrackingID_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	es := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupEmailTest(es, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/emails/trk_nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmailHandler_GetByTrackingID_WrongWorkspace(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	otherWsID := uuid.Must(uuid.NewV7())
	es := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return &domain.Email{
				ID: uuid.Must(uuid.NewV7()), TrackingID: "trk_abc", WorkspaceID: otherWsID,
				TenantID: uuid.Must(uuid.NewV7()), Status: domain.StatusSent,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupEmailTest(es, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/emails/trk_abc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmailHandler_GetEvents_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	emailID := uuid.Must(uuid.NewV7())
	es := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return &domain.Email{
				ID: emailID, TrackingID: "trk_abc", WorkspaceID: ws.ID, TenantID: ws.TenantID,
				Status: domain.StatusSent, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		getEventsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.EmailEvent, error) {
			return []*domain.EmailEvent{
				{ID: uuid.Must(uuid.NewV7()), EmailID: emailID, EventType: domain.EventTypeQueued, OccurredAt: now, CreatedAt: now},
				{ID: uuid.Must(uuid.NewV7()), EmailID: emailID, EventType: domain.EventTypeSent, OccurredAt: now, CreatedAt: now},
			}, nil
		},
	}

	e, _ := setupEmailTest(es, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/emails/trk_abc/events", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var events []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&events); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestEmailHandler_List_InvalidTenant(t *testing.T) {
	es := &mockEmailStore{}
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupEmailTest(es, ts, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/nonexistent/workspaces/default/emails", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

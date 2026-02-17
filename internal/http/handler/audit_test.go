package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

// --- Mock AuditLogStore ---

type mockAuditLogStore struct {
	appendFn func(ctx context.Context, entry *domain.AuditLog) error
	queryFn  func(ctx context.Context, filter port.AuditFilter, opts port.ListOptions) (*port.PageResult[domain.AuditLog], error)
}

func (m *mockAuditLogStore) Append(ctx context.Context, entry *domain.AuditLog) error {
	if m.appendFn != nil {
		return m.appendFn(ctx, entry)
	}
	return nil
}
func (m *mockAuditLogStore) Query(ctx context.Context, filter port.AuditFilter, opts port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, filter, opts)
	}
	return &port.PageResult[domain.AuditLog]{Items: []*domain.AuditLog{}}, nil
}

// --- Helpers ---

func setupAuditTest(als port.AuditLogStore, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.AuditHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewAuditHandler(als, ts, ws)

	base := "/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code"
	e.GET(base+"/audit-log", h.Query)
	e.GET("/api/v1/manage/global/audit-log", h.QueryGlobal)

	return e, h
}

// --- Tests ---

func TestAuditHandler_Query_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	als := &mockAuditLogStore{
		queryFn: func(_ context.Context, f port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
			// Verify workspace scoping.
			if f.WorkspaceID == nil || *f.WorkspaceID != ws.ID {
				t.Fatalf("expected workspace filter %s", ws.ID)
			}
			return &port.PageResult[domain.AuditLog]{
				Items: []*domain.AuditLog{
					{
						ID: uuid.Must(uuid.NewV7()), ActorID: uuid.Must(uuid.NewV7()),
						ActorEmail: "admin@acme.com", Action: domain.AuditCreate,
						EntityType: "template", EntityID: uuid.Must(uuid.NewV7()),
						TenantID: &ws.TenantID, WorkspaceID: &ws.ID, ScopeType: domain.ScopeWorkspace,
						CreatedAt: now,
					},
				},
				HasMore: false,
			}, nil
		},
	}

	e, _ := setupAuditTest(als, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/audit-log", nil)
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
	if items[0]["action"] != "create" {
		t.Fatalf("expected action 'create', got %v", items[0]["action"])
	}
}

func TestAuditHandler_Query_WithFilters(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var capturedFilter port.AuditFilter
	als := &mockAuditLogStore{
		queryFn: func(_ context.Context, f port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
			capturedFilter = f
			return &port.PageResult[domain.AuditLog]{Items: []*domain.AuditLog{}}, nil
		},
	}

	e, _ := setupAuditTest(als, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/audit-log?action=create&entity_type=template&since=2024-01-01T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if capturedFilter.Action == nil || *capturedFilter.Action != "create" {
		t.Fatal("expected action filter 'create'")
	}
	if capturedFilter.EntityType == nil || *capturedFilter.EntityType != "template" {
		t.Fatal("expected entity_type filter 'template'")
	}
	if capturedFilter.Since == nil {
		t.Fatal("expected since filter")
	}
}

func TestAuditHandler_QueryGlobal_Success(t *testing.T) {
	now := time.Now().UTC()
	als := &mockAuditLogStore{
		queryFn: func(_ context.Context, f port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
			// Global query should NOT have workspace filter set from URL resolution.
			if f.WorkspaceID != nil {
				t.Fatal("expected nil workspace filter for global query")
			}
			return &port.PageResult[domain.AuditLog]{
				Items: []*domain.AuditLog{
					{
						ID: uuid.Must(uuid.NewV7()), ActorID: uuid.Must(uuid.NewV7()),
						ActorEmail: "admin@acme.com", Action: domain.AuditCreate,
						EntityType: "tenant", EntityID: uuid.Must(uuid.NewV7()),
						ScopeType: domain.ScopeGlobal, CreatedAt: now,
					},
				},
				HasMore: false,
			}, nil
		},
	}

	e, _ := setupAuditTest(als, &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/audit-log", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuditHandler_Query_EmptyResults(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	als := &mockAuditLogStore{
		queryFn: func(_ context.Context, _ port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
			return &port.PageResult[domain.AuditLog]{Items: []*domain.AuditLog{}}, nil
		},
	}

	e, _ := setupAuditTest(als, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/audit-log", nil)
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
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestAuditHandler_Query_InvalidTenant(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupAuditTest(&mockAuditLogStore{}, ts, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/nonexistent/workspaces/default/audit-log", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuditHandler_Query_Pagination(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var capturedOpts port.ListOptions
	als := &mockAuditLogStore{
		queryFn: func(_ context.Context, _ port.AuditFilter, opts port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
			capturedOpts = opts
			return &port.PageResult[domain.AuditLog]{
				Items:      []*domain.AuditLog{},
				NextCursor: "next_page",
				HasMore:    true,
			}, nil
		},
	}

	e, _ := setupAuditTest(als, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/audit-log?limit=10&cursor=prev_cursor", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedOpts.Limit != 10 {
		t.Fatalf("expected limit 10, got %d", capturedOpts.Limit)
	}
	if capturedOpts.Cursor != "prev_cursor" {
		t.Fatalf("expected cursor prev_cursor, got %s", capturedOpts.Cursor)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["has_more"] != true {
		t.Fatal("expected has_more=true")
	}
	if resp["next_cursor"] != "next_page" {
		t.Fatalf("expected next_cursor 'next_page', got %v", resp["next_cursor"])
	}
}

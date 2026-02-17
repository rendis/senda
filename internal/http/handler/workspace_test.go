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
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

func setupWorkspaceTest(ts port.TenantStore, ws port.WorkspaceStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewWorkspaceHandler(ts, ws)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code", h.Update)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code", h.SoftDelete)
	return e
}

func TestWorkspaceHandler_Create_Success(t *testing.T) {
	tenantID := uuid.New()
	var createdWS *domain.Workspace

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &mockWorkspaceStore{
		createFn: func(_ context.Context, w *domain.Workspace) error {
			createdWS = w
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws)

	body := `{"code":"main","name":"Main Workspace"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.WorkspaceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "main" {
		t.Fatalf("expected code 'main', got %q", resp.Code)
	}
	if resp.TenantID != tenantID.String() {
		t.Fatalf("expected tenant_id %q, got %q", tenantID.String(), resp.TenantID)
	}
	if createdWS == nil {
		t.Fatal("expected workspace to be created")
	}
	if createdWS.TenantID != tenantID {
		t.Fatalf("expected tenant ID %s, got %s", tenantID, createdWS.TenantID)
	}
}

func TestWorkspaceHandler_Create_InvalidSlug(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: uuid.New(), Code: code}, nil
		},
	}

	e := setupWorkspaceTest(ts, &mockWorkspaceStore{})

	body := `{"code":"AB","name":"Bad Slug"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandler_Create_TenantNotFound(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}

	e := setupWorkspaceTest(ts, &mockWorkspaceStore{})

	body := `{"code":"main","name":"Main"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/nonexistent/workspaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandler_List_Success(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now().UTC()
	workspaces := []*domain.Workspace{
		{ID: uuid.New(), TenantID: tenantID, Code: "main", Name: "Main", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Code: "staging", Name: "Staging", CreatedAt: now, UpdatedAt: now},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		listByTenantFn: func(_ context.Context, tid uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant ID %s, got %s", tenantID, tid)
			}
			return workspaces, "", nil
		},
	}

	e := setupWorkspaceTest(ts, ws)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.WorkspaceListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.HasMore {
		t.Fatal("expected has_more=false when next_cursor is empty")
	}
}

func TestWorkspaceHandler_Get_Success(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	now := time.Now().UTC()

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID: wsID, TenantID: tid, Code: code, Name: "Main",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e := setupWorkspaceTest(ts, ws)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/main", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.WorkspaceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "main" {
		t.Fatalf("expected code 'main', got %q", resp.Code)
	}
}

func TestWorkspaceHandler_Update_Success(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	now := time.Now().UTC()

	var updatedName string
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID: wsID, TenantID: tenantID, Code: "main", Name: "Main",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		updateFn: func(_ context.Context, w *domain.Workspace) error {
			updatedName = w.Name
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws)

	body := `{"name":"Production"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/main", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updatedName != "Production" {
		t.Fatalf("expected updated name 'Production', got %q", updatedName)
	}
}

func TestWorkspaceHandler_SoftDelete_Success(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	now := time.Now().UTC()

	var deletedID uuid.UUID
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID: wsID, TenantID: tenantID, Code: "staging", Name: "Staging",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		softDeleteFn: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/staging", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if deletedID != wsID {
		t.Fatalf("expected deleted ID %s, got %s", wsID, deletedID)
	}
}

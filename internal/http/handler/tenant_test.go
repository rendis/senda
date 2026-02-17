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

// --- Mock Stores ---

type mockTenantStore struct {
	createFn     func(ctx context.Context, t *domain.Tenant) error
	getByIDFn    func(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	getByCodeFn  func(ctx context.Context, code string) (*domain.Tenant, error)
	listFn       func(ctx context.Context, opts port.ListOptions) ([]*domain.Tenant, string, error)
	updateFn     func(ctx context.Context, t *domain.Tenant) error
	softDeleteFn func(ctx context.Context, id uuid.UUID) error
}

func (m *mockTenantStore) Create(ctx context.Context, t *domain.Tenant) error {
	if m.createFn != nil {
		return m.createFn(ctx, t)
	}
	return nil
}
func (m *mockTenantStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockTenantStore) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	if m.getByCodeFn != nil {
		return m.getByCodeFn(ctx, code)
	}
	return nil, nil
}
func (m *mockTenantStore) List(ctx context.Context, opts port.ListOptions) ([]*domain.Tenant, string, error) {
	if m.listFn != nil {
		return m.listFn(ctx, opts)
	}
	return nil, "", nil
}
func (m *mockTenantStore) Update(ctx context.Context, t *domain.Tenant) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, t)
	}
	return nil
}
func (m *mockTenantStore) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockTenantStore) Purge(_ context.Context, _ uuid.UUID) error { return nil }

type mockWorkspaceStore struct {
	createFn             func(ctx context.Context, ws *domain.Workspace) error
	getByIDFn            func(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	getByTenantAndCodeFn func(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error)
	getSystemWorkspaceFn func(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error)
	listByTenantFn       func(ctx context.Context, tenantID uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error)
	updateFn             func(ctx context.Context, ws *domain.Workspace) error
	softDeleteFn         func(ctx context.Context, id uuid.UUID) error
}

func (m *mockWorkspaceStore) Create(ctx context.Context, ws *domain.Workspace) error {
	if m.createFn != nil {
		return m.createFn(ctx, ws)
	}
	return nil
}
func (m *mockWorkspaceStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockWorkspaceStore) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
	if m.getByTenantAndCodeFn != nil {
		return m.getByTenantAndCodeFn(ctx, tenantID, code)
	}
	return nil, nil
}
func (m *mockWorkspaceStore) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
	if m.getSystemWorkspaceFn != nil {
		return m.getSystemWorkspaceFn(ctx, tenantID)
	}
	return nil, nil
}
func (m *mockWorkspaceStore) ListByTenant(ctx context.Context, tenantID uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error) {
	if m.listByTenantFn != nil {
		return m.listByTenantFn(ctx, tenantID, opts)
	}
	return nil, "", nil
}
func (m *mockWorkspaceStore) Update(ctx context.Context, ws *domain.Workspace) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, ws)
	}
	return nil
}
func (m *mockWorkspaceStore) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

// --- Helper ---

func setupTenantTest(ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.TenantHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewTenantHandler(ts, ws)
	e.POST("/api/v1/manage/tenants", h.Create)
	e.GET("/api/v1/manage/tenants", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code", h.GetByCode)
	e.PUT("/api/v1/manage/tenants/:tenant_code", h.Update)
	e.DELETE("/api/v1/manage/tenants/:tenant_code", h.SoftDelete)
	return e, h
}

// --- Tests ---

func TestTenantHandler_Create_Success(t *testing.T) {
	var createdTenant *domain.Tenant
	var createdWS *domain.Workspace

	ts := &mockTenantStore{
		createFn: func(_ context.Context, t *domain.Tenant) error {
			createdTenant = t
			return nil
		},
	}
	ws := &mockWorkspaceStore{
		createFn: func(_ context.Context, w *domain.Workspace) error {
			createdWS = w
			return nil
		},
	}

	e, _ := setupTenantTest(ts, ws)

	body := `{"code":"acme","name":"Acme Corp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TenantResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "acme" {
		t.Fatalf("expected code 'acme', got %q", resp.Code)
	}
	if resp.Name != "Acme Corp" {
		t.Fatalf("expected name 'Acme Corp', got %q", resp.Name)
	}
	if createdTenant == nil {
		t.Fatal("expected tenant to be created in store")
	}
	if createdWS == nil {
		t.Fatal("expected _system workspace to be created")
	}
	if !createdWS.IsSystem {
		t.Fatal("expected _system workspace to have IsSystem=true")
	}
	if createdWS.Code != "_system" {
		t.Fatalf("expected workspace code '_system', got %q", createdWS.Code)
	}
}

func TestTenantHandler_Create_InvalidSlug(t *testing.T) {
	e, _ := setupTenantTest(&mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"code":"AB","name":"Too short and uppercase"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantHandler_Create_MissingName(t *testing.T) {
	e, _ := setupTenantTest(&mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"code":"acme","name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantHandler_Create_Conflict(t *testing.T) {
	ts := &mockTenantStore{
		createFn: func(_ context.Context, _ *domain.Tenant) error {
			return domain.ErrConflict
		},
	}

	e, _ := setupTenantTest(ts, &mockWorkspaceStore{})

	body := `{"code":"acme","name":"Acme Corp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantHandler_List_Success(t *testing.T) {
	now := time.Now().UTC()
	tenants := []*domain.Tenant{
		{ID: uuid.New(), Code: "acme", Name: "Acme Corp", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), Code: "globex", Name: "Globex", CreatedAt: now, UpdatedAt: now},
	}

	ts := &mockTenantStore{
		listFn: func(_ context.Context, opts port.ListOptions) ([]*domain.Tenant, string, error) {
			if opts.Limit != 25 {
				t.Fatalf("expected default limit 25, got %d", opts.Limit)
			}
			return tenants, "next-cursor-token", nil
		},
	}

	e, _ := setupTenantTest(ts, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TenantListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.NextCursor != "next-cursor-token" {
		t.Fatalf("expected next_cursor 'next-cursor-token', got %q", resp.NextCursor)
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
}

func TestTenantHandler_GetByCode_Success(t *testing.T) {
	now := time.Now().UTC()
	tenant := &domain.Tenant{ID: uuid.New(), Code: "acme", Name: "Acme Corp", CreatedAt: now, UpdatedAt: now}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			if code != "acme" {
				t.Fatalf("expected code 'acme', got %q", code)
			}
			return tenant, nil
		},
	}

	e, _ := setupTenantTest(ts, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TenantResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "acme" {
		t.Fatalf("expected code 'acme', got %q", resp.Code)
	}
}

func TestTenantHandler_GetByCode_NotFound(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupTenantTest(ts, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantHandler_Update_Success(t *testing.T) {
	now := time.Now().UTC()
	tenant := &domain.Tenant{ID: uuid.New(), Code: "acme", Name: "Acme Corp", CreatedAt: now, UpdatedAt: now}

	var updatedName string
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return tenant, nil
		},
		updateFn: func(_ context.Context, t *domain.Tenant) error {
			updatedName = t.Name
			return nil
		},
	}

	e, _ := setupTenantTest(ts, &mockWorkspaceStore{})

	body := `{"name":"Acme Industries"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updatedName != "Acme Industries" {
		t.Fatalf("expected updated name 'Acme Industries', got %q", updatedName)
	}
}

func TestTenantHandler_SoftDelete_Success(t *testing.T) {
	now := time.Now().UTC()
	tenant := &domain.Tenant{ID: uuid.New(), Code: "acme", Name: "Acme Corp", CreatedAt: now, UpdatedAt: now}

	var deletedID uuid.UUID
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return tenant, nil
		},
		softDeleteFn: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	e, _ := setupTenantTest(ts, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if deletedID != tenant.ID {
		t.Fatalf("expected deleted ID %s, got %s", tenant.ID, deletedID)
	}
}

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
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

func setupWorkspaceTest(ts port.TenantStore, ws port.WorkspaceStore, emails port.EmailStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewWorkspaceHandler(ts, ws, emails)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code", h.Update)
	e.GET("/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code", h.Get)
	e.PUT("/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code", h.Update)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code", h.SoftDelete)
	e.POST("/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/runtime/reset", h.ResetRuntime)
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

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

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
	if !createdWS.IsActive {
		t.Fatal("expected created workspace to be active by default")
	}
	if !resp.IsActive {
		t.Fatal("expected response is_active=true")
	}
}

func TestWorkspaceHandler_Create_InvalidSlug(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: uuid.New(), Code: code}, nil
		},
	}

	e := setupWorkspaceTest(ts, &mockWorkspaceStore{}, &mockEmailStore{})

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

	e := setupWorkspaceTest(ts, &mockWorkspaceStore{}, &mockEmailStore{})

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
		{ID: uuid.New(), TenantID: tenantID, Code: "_system", Name: "System", IsSystem: true, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Code: "main", Name: "Main", IsActive: false, CreatedAt: now, UpdatedAt: now},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		listByTenantFn: func(_ context.Context, tid uuid.UUID, _ domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant ID %s, got %s", tenantID, tid)
			}
			return workspaces, "", nil
		},
	}

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

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
	if !resp.Items[0].IsActive {
		t.Fatal("expected first workspace to be active")
	}
	if resp.Items[1].IsActive {
		t.Fatal("expected second workspace to be inactive")
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
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID: wsID, TenantID: tid, Code: code, Name: "Main", IsActive: true,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

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
	if !resp.IsActive {
		t.Fatal("expected is_active=true")
	}
}

func TestWorkspaceHandler_Update_SharedFieldsSuccess(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	now := time.Now().UTC()

	var sharedTenantID uuid.UUID
	var sharedCurrentCode string
	var sharedNextCode string
	var updatedName string
	var updateCalled bool
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID: wsID, TenantID: tenantID, Code: "main", Name: "Main", IsActive: true,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		updateSharedFn: func(_ context.Context, tenantID uuid.UUID, currentCode, nextCode, nextName string) error {
			sharedTenantID = tenantID
			sharedCurrentCode = currentCode
			sharedNextCode = nextCode
			updatedName = nextName
			return nil
		},
		updateFn: func(_ context.Context, w *domain.Workspace) error {
			updateCalled = true
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

	body := `{"name":"Production","code":"prod-main"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/main", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if sharedTenantID != tenantID {
		t.Fatalf("expected shared update tenant %s, got %s", tenantID, sharedTenantID)
	}
	if sharedCurrentCode != "main" || sharedNextCode != "prod-main" {
		t.Fatalf("expected shared code update main -> prod-main, got %q -> %q", sharedCurrentCode, sharedNextCode)
	}
	if updatedName != "Production" {
		t.Fatalf("expected shared updated name 'Production', got %q", updatedName)
	}
	if updateCalled {
		t.Fatal("expected shared-only update to skip environment row mutation")
	}
}

func TestWorkspaceHandler_Update_SharedRouteRejectsEnvironmentFields(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	now := time.Now().UTC()
	defaultLocale := "en"

	var sharedUpdateCalled bool
	var environmentUpdateCalled bool
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID:                  wsID,
				TenantID:            tenantID,
				Code:                "main",
				Name:                "Main",
				IsActive:            true,
				OpenTrackingEnabled: true,
				DefaultLocale:       &defaultLocale,
				CreatedAt:           now,
				UpdatedAt:           now,
			}, nil
		},
		updateSharedFn: func(_ context.Context, _ uuid.UUID, _, _, _ string) error {
			sharedUpdateCalled = true
			return nil
		},
		updateFn: func(_ context.Context, _ *domain.Workspace) error {
			environmentUpdateCalled = true
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

	body := `{"is_active":false,"open_tracking_enabled":false,"default_locale":"es"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/main", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if sharedUpdateCalled {
		t.Fatal("expected shared route to reject environment fields before shared update")
	}
	if environmentUpdateCalled {
		t.Fatal("expected shared route to reject environment fields before environment update")
	}
}

func TestWorkspaceHandler_Update_EnvironmentRouteAllowsEnvironmentFields(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	now := time.Now().UTC()
	defaultLocale := "en"

	var updatedWorkspace *domain.Workspace
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, environment domain.Environment) (*domain.Workspace, error) {
			if environment != domain.EnvironmentTest {
				return nil, domain.ErrNotFound
			}
			return &domain.Workspace{
				ID:                  wsID,
				TenantID:            tenantID,
				Code:                "main",
				Name:                "Main",
				Environment:         domain.EnvironmentTest,
				IsActive:            true,
				OpenTrackingEnabled: true,
				DefaultLocale:       &defaultLocale,
				CreatedAt:           now,
				UpdatedAt:           now,
			}, nil
		},
		updateFn: func(_ context.Context, w *domain.Workspace) error {
			copy := *w
			updatedWorkspace = &copy
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

	body := `{"is_active":false,"open_tracking_enabled":false,"default_locale":"es"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/environments/test/tenants/acme/workspaces/main", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updatedWorkspace == nil {
		t.Fatal("expected environment route to persist workspace row changes")
	}
	if updatedWorkspace.IsActive {
		t.Fatal("expected environment route to disable workspace")
	}
	if updatedWorkspace.OpenTrackingEnabled {
		t.Fatal("expected environment route to disable open tracking")
	}
	if updatedWorkspace.DefaultLocale == nil || *updatedWorkspace.DefaultLocale != "es" {
		t.Fatalf("expected default locale es, got %v", updatedWorkspace.DefaultLocale)
	}
}

func TestWorkspaceHandler_Update_SystemWorkspaceBlocked(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now().UTC()
	var updateCalled bool

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID: uuid.New(), TenantID: tenantID, Code: "_system", Name: "System", IsSystem: true, IsActive: true,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		updateFn: func(_ context.Context, _ *domain.Workspace) error {
			updateCalled = true
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

	body := `{"name":"Renamed System","is_active":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/_system", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if updateCalled {
		t.Fatal("expected protected system workspace to skip update")
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
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
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

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

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

func TestWorkspaceHandler_SoftDelete_SystemWorkspaceBlocked(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now().UTC()
	var softDeleteCalled bool

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID: uuid.New(), TenantID: tenantID, Code: "_system", Name: "System", IsSystem: true, IsActive: true,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		softDeleteFn: func(_ context.Context, _ uuid.UUID) error {
			softDeleteCalled = true
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws, &mockEmailStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/_system", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if softDeleteCalled {
		t.Fatal("expected protected system workspace to skip delete")
	}
}

func TestWorkspaceHandler_ResetRuntime_TestEnvironment_Success(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	now := time.Now().UTC()

	var purgedWorkspaceID uuid.UUID
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
			if code != "staging" || environment != domain.EnvironmentTest {
				return nil, domain.ErrNotFound
			}
			return &domain.Workspace{
				ID:          wsID,
				TenantID:    tenantID,
				Code:        "staging",
				Name:        "Staging",
				Environment: domain.EnvironmentTest,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}
	emails := &mockEmailStore{
		purgeWorkspaceFn: func(_ context.Context, workspaceID uuid.UUID) error {
			purgedWorkspaceID = workspaceID
			return nil
		},
	}

	e := setupWorkspaceTest(ts, ws, emails)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/environments/test/tenants/acme/workspaces/staging/runtime/reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if purgedWorkspaceID != wsID {
		t.Fatalf("expected purge for workspace %s, got %s", wsID, purgedWorkspaceID)
	}
}

func TestWorkspaceHandler_ResetRuntime_ProdBlocked(t *testing.T) {
	tenantID := uuid.New()
	var purgeCalled bool

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	emails := &mockEmailStore{
		purgeWorkspaceFn: func(_ context.Context, _ uuid.UUID) error {
			purgeCalled = true
			return nil
		},
	}

	e := setupWorkspaceTest(ts, &mockWorkspaceStore{}, emails)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/environments/prod/tenants/acme/workspaces/staging/runtime/reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if purgeCalled {
		t.Fatal("expected prod runtime reset to skip purge")
	}
}

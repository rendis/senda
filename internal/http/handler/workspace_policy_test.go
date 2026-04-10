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
)

func setupWorkspacePolicyTest(ts *mockTenantStore, ws *mockWorkspaceStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewWorkspacePolicyHandler(ts, ws)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/policies", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/policies", h.Update)
	return e
}

func TestWorkspacePolicyHandler_Get_SystemWorkspaceSuccess(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now().UTC()

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tid)
			}
			if code != "_system" {
				t.Fatalf("expected _system workspace, got %q", code)
			}
			return &domain.Workspace{
				ID:                                   uuid.New(),
				TenantID:                             tenantID,
				Code:                                 "_system",
				Name:                                 "Default",
				IsSystem:                             true,
				AllowWorkspaceLocalTemplates:         false,
				AllowWorkspaceInheritedTemplateForks: true,
				AllowWorkspaceLocalInjectors:         false,
				WorkspacePoliciesInitialized:         true,
				CreatedAt:                            now,
				UpdatedAt:                            now,
			}, nil
		},
	}

	e := setupWorkspacePolicyTest(ts, ws)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/_system/policies", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp handler.WorkspacePolicyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AllowWorkspaceLocalTemplates {
		t.Fatal("expected local templates disabled")
	}
	if !resp.AllowWorkspaceInheritedTemplateForks {
		t.Fatal("expected inherited template forks enabled")
	}
	if resp.AllowWorkspaceLocalInjectors {
		t.Fatal("expected local injectors disabled")
	}
}

func TestWorkspacePolicyHandler_Get_WorkspaceReturnsSystemPolicies(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now().UTC()
	systemWorkspaceID := uuid.New()

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tid)
			}
			if code != "main" {
				t.Fatalf("expected main workspace, got %q", code)
			}
			return &domain.Workspace{
				ID:        uuid.New(),
				TenantID:  tenantID,
				Code:      "main",
				Name:      "Main",
				IsSystem:  false,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
		getSystemWorkspaceFn: func(_ context.Context, tid uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tid)
			}
			return &domain.Workspace{
				ID:                                   systemWorkspaceID,
				TenantID:                             tenantID,
				Code:                                 "_system",
				Name:                                 "System",
				IsSystem:                             true,
				AllowWorkspaceLocalTemplates:         false,
				AllowWorkspaceInheritedTemplateForks: true,
				AllowWorkspaceLocalInjectors:         false,
				WorkspacePoliciesInitialized:         true,
				CreatedAt:                            now,
				UpdatedAt:                            now,
			}, nil
		},
	}

	e := setupWorkspacePolicyTest(ts, ws)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/main/policies", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp handler.WorkspacePolicyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AllowWorkspaceLocalTemplates {
		t.Fatal("expected local templates disabled")
	}
	if !resp.AllowWorkspaceInheritedTemplateForks {
		t.Fatal("expected inherited template forks enabled")
	}
	if resp.AllowWorkspaceLocalInjectors {
		t.Fatal("expected local injectors disabled")
	}
}

func TestWorkspacePolicyHandler_Update_SystemWorkspaceSuccess(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now().UTC()
	systemWorkspace := &domain.Workspace{
		ID:                                   uuid.New(),
		TenantID:                             tenantID,
		Code:                                 "_system",
		Name:                                 "Default",
		IsSystem:                             true,
		AllowWorkspaceLocalTemplates:         true,
		AllowWorkspaceInheritedTemplateForks: true,
		AllowWorkspaceLocalInjectors:         true,
		WorkspacePoliciesInitialized:         true,
		CreatedAt:                            now,
		UpdatedAt:                            now,
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	var updated *domain.Workspace
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID || code != "_system" {
				t.Fatalf("unexpected lookup %s %q", tid, code)
			}
			copy := *systemWorkspace
			return &copy, nil
		},
		updateFn: func(_ context.Context, ws *domain.Workspace) error {
			updated = ws
			return nil
		},
	}

	e := setupWorkspacePolicyTest(ts, ws)

	body := `{"allow_workspace_local_templates":false,"allow_workspace_inherited_template_forks":true,"allow_workspace_local_injectors":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/_system/policies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated == nil {
		t.Fatal("expected workspace update")
	}
	if updated.AllowWorkspaceLocalTemplates {
		t.Fatal("expected local templates disabled")
	}
	if !updated.AllowWorkspaceInheritedTemplateForks {
		t.Fatal("expected template forks enabled")
	}
	if updated.AllowWorkspaceLocalInjectors {
		t.Fatal("expected local injectors disabled")
	}
}

func TestWorkspacePolicyHandler_Update_RejectsNonSystemWorkspace(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now().UTC()

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	var updateCalled bool
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{
				ID:        uuid.New(),
				TenantID:  tid,
				Code:      code,
				Name:      "Main",
				IsSystem:  false,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
		updateFn: func(_ context.Context, _ *domain.Workspace) error {
			updateCalled = true
			return nil
		},
	}

	e := setupWorkspacePolicyTest(ts, ws)

	body := `{"allow_workspace_local_templates":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/main/policies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if updateCalled {
		t.Fatal("expected non-system workspace policies update to be blocked")
	}
}

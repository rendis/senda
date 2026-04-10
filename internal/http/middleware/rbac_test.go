package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/port"
)

// --- Manual Mocks for RBAC ---

type mockTenantStore struct {
	getByCodeFn func(ctx context.Context, code string) (*domain.Tenant, error)
}

func (m *mockTenantStore) Create(_ context.Context, _ *domain.Tenant) error { return nil }
func (m *mockTenantStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (m *mockTenantStore) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	return m.getByCodeFn(ctx, code)
}
func (m *mockTenantStore) List(_ context.Context, _ port.ListOptions) ([]*domain.Tenant, string, error) {
	return nil, "", nil
}
func (m *mockTenantStore) Update(_ context.Context, _ *domain.Tenant) error    { return nil }
func (m *mockTenantStore) SoftDelete(_ context.Context, _ uuid.UUID) error     { return nil }
func (m *mockTenantStore) Purge(_ context.Context, _ uuid.UUID) error          { return nil }

type mockWorkspaceStore struct {
	getByTenantAndCodeFn func(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error)
}

func (m *mockWorkspaceStore) Create(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStore) CreateLogicalPair(_ context.Context, _ *domain.Workspace, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStore) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
	return m.getByTenantAndCodeFn(ctx, tenantID, code, environment)
}
func (m *mockWorkspaceStore) GetSystemWorkspace(_ context.Context, _ uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStore) ListByTenant(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
	return nil, "", nil
}
func (m *mockWorkspaceStore) UpdateShared(_ context.Context, _ uuid.UUID, _, _, _ string) error { return nil }
func (m *mockWorkspaceStore) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStore) SoftDeleteLogical(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockWorkspaceStore) SoftDelete(_ context.Context, _ uuid.UUID) error     { return nil }

// --- Helper: build Echo with pre-set context values ---

func setupRBACTest(
	authType string,
	roles []*domain.MemberRole,
	tenantCode, workspaceCode string,
	tenantStore port.TenantStore,
	wsStore port.WorkspaceStore,
	minRole domain.Role,
) (*httptest.ResponseRecorder, int) {
	e := echo.New()

	// Simulate Auth and Scope middleware by pre-setting context values.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if authType != "" {
				c.Set(middleware.ContextKeyAuthType, authType)
			}
			if roles != nil {
				c.Set(middleware.ContextKeyRoles, roles)
			}
			if tenantCode != "" {
				c.Set(middleware.ContextKeyTenantCode, tenantCode)
			}
			if workspaceCode != "" {
				c.Set(middleware.ContextKeyWorkspaceCode, workspaceCode)
			}
			return next(c)
		}
	})
	e.Use(middleware.RequireRole(minRole, tenantStore, wsStore))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec, rec.Code
}

// --- Tests ---

func TestRequireRole_APIKeyBypassesRBAC(t *testing.T) {
	_, code := setupRBACTest(
		"apikey",
		nil, // no roles for API keys
		"", "",
		&mockTenantStore{}, &mockWorkspaceStore{},
		domain.RoleWorkspaceAdmin,
	)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestRequireRole_SuperadminAccessAnything(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()

	roles := []*domain.MemberRole{
		{
			ID:        uuid.New(),
			Role:      domain.RoleSuperadmin,
			ScopeType: domain.ScopeGlobal,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: wsID, TenantID: tenantID, Code: "main"}, nil
		},
	}

	_, code := setupRBACTest("oidc", roles, "acme", "main", ts, ws, domain.RoleWorkspaceAdmin)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestRequireRole_SuperadminWrongScopeType(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()

	// Superadmin assigned at workspace scope (data integrity error) should NOT bypass.
	roles := []*domain.MemberRole{
		{
			ID:          uuid.New(),
			Role:        domain.RoleSuperadmin,
			ScopeType:   domain.ScopeWorkspace,
			WorkspaceID: &wsID,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: wsID, TenantID: tenantID, Code: "main"}, nil
		},
	}

	// Superadmin at workspace scope should NOT get global bypass — only workspace-level access
	_, code := setupRBACTest("oidc", roles, "acme", "main", ts, ws, domain.RoleWorkspaceAdmin)
	if code != http.StatusOK {
		t.Fatalf("expected 200 (workspace-scoped match), got %d", code)
	}

	// But it should NOT bypass into a different workspace
	otherWsID := uuid.New()
	wsOther := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: otherWsID, TenantID: tenantID, Code: "other"}, nil
		},
	}
	_, code = setupRBACTest("oidc", roles, "acme", "other", ts, wsOther, domain.RoleWorkspaceAdmin)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403 (superadmin at wrong scope should not bypass), got %d", code)
	}
}

func TestRequireRole_TenantAdminOwnTenant(t *testing.T) {
	tenantID := uuid.New()

	roles := []*domain.MemberRole{
		{
			ID:        uuid.New(),
			Role:      domain.RoleTenantAdmin,
			ScopeType: domain.ScopeTenant,
			TenantID:  &tenantID,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}

	_, code := setupRBACTest("oidc", roles, "acme", "", ts, &mockWorkspaceStore{}, domain.RoleTenantAdmin)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestRequireRole_TenantAdminOtherTenant(t *testing.T) {
	tenantID := uuid.New()
	otherTenantID := uuid.New()

	roles := []*domain.MemberRole{
		{
			ID:        uuid.New(),
			Role:      domain.RoleTenantAdmin,
			ScopeType: domain.ScopeTenant,
			TenantID:  &otherTenantID,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}

	_, code := setupRBACTest("oidc", roles, "acme", "", ts, &mockWorkspaceStore{}, domain.RoleTenantAdmin)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestRequireRole_WorkspaceAdminOwnWorkspace(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()

	roles := []*domain.MemberRole{
		{
			ID:          uuid.New(),
			Role:        domain.RoleWorkspaceAdmin,
			ScopeType:   domain.ScopeWorkspace,
			WorkspaceID: &wsID,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: wsID, TenantID: tenantID, Code: "main"}, nil
		},
	}

	_, code := setupRBACTest("oidc", roles, "acme", "main", ts, ws, domain.RoleWorkspaceAdmin)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestRequireRole_ViewerCannotAccessEditorEndpoint(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()

	roles := []*domain.MemberRole{
		{
			ID:          uuid.New(),
			Role:        domain.RoleWorkspaceViewer,
			ScopeType:   domain.ScopeWorkspace,
			WorkspaceID: &wsID,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: wsID, TenantID: tenantID, Code: "main"}, nil
		},
	}

	_, code := setupRBACTest("oidc", roles, "acme", "main", ts, ws, domain.RoleWorkspaceEditor)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestRequireRole_NoMatchingRoleForScope(t *testing.T) {
	tenantID := uuid.New()
	otherWsID := uuid.New()
	requestedWsID := uuid.New()

	roles := []*domain.MemberRole{
		{
			ID:          uuid.New(),
			Role:        domain.RoleWorkspaceAdmin,
			ScopeType:   domain.ScopeWorkspace,
			WorkspaceID: &otherWsID,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: requestedWsID, TenantID: tenantID, Code: "other"}, nil
		},
	}

	_, code := setupRBACTest("oidc", roles, "acme", "other", ts, ws, domain.RoleWorkspaceAdmin)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestRequireRole_MissingAuthContext(t *testing.T) {
	// No auth middleware ran — no auth_type in context.
	_, code := setupRBACTest("", nil, "", "", &mockTenantStore{}, &mockWorkspaceStore{}, domain.RoleWorkspaceAdmin)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestRequireRole_TenantAdminAccessWorkspaceInOwnTenant(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()

	roles := []*domain.MemberRole{
		{
			ID:        uuid.New(),
			Role:      domain.RoleTenantAdmin,
			ScopeType: domain.ScopeTenant,
			TenantID:  &tenantID,
		},
	}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: "acme"}, nil
		},
	}
	ws := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: wsID, TenantID: tid, Code: "main"}, nil
		},
	}

	// TenantAdmin should be able to access a workspace within their own tenant.
	_, code := setupRBACTest("oidc", roles, "acme", "main", ts, ws, domain.RoleWorkspaceAdmin)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

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

type mockMemberStore struct {
	createFn            func(ctx context.Context, m *domain.Member) error
	getByEmailFn        func(ctx context.Context, email string) (*domain.Member, error)
	getByIDFn           func(ctx context.Context, id uuid.UUID) (*domain.Member, error)
	countAllFn          func(ctx context.Context) (int64, error)
	listAllFn           func(ctx context.Context, opts port.ListOptions) ([]*domain.Member, string, error)
	listInScopeFn       func(ctx context.Context, scopeType domain.ScopeType, scopeID *uuid.UUID, opts port.ListOptions) ([]*domain.Member, string, error)
	addRoleFn           func(ctx context.Context, role *domain.MemberRole) error
	replaceRoleFn       func(ctx context.Context, role *domain.MemberRole) error
	removeRoleFn        func(ctx context.Context, roleID uuid.UUID) error
	revokeAccessFn      func(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error)
	getRolesFn          func(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error)
	getRolesInScopeFn   func(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error)
	getRolesByMembersFn func(ctx context.Context, memberIDs []uuid.UUID) (map[uuid.UUID][]*domain.MemberRole, error)
}

func (m *mockMemberStore) Create(ctx context.Context, member *domain.Member) error {
	if m.createFn != nil {
		return m.createFn(ctx, member)
	}
	return nil
}
func (m *mockMemberStore) GetByEmail(ctx context.Context, email string) (*domain.Member, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return nil, nil
}
func (m *mockMemberStore) GetByOIDCIdentity(_ context.Context, _ string, _ string) (*domain.Member, error) {
	return nil, nil
}
func (m *mockMemberStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Member, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockMemberStore) CountAll(ctx context.Context) (int64, error) {
	if m.countAllFn != nil {
		return m.countAllFn(ctx)
	}
	return 0, nil
}
func (m *mockMemberStore) ListAll(ctx context.Context, opts port.ListOptions) ([]*domain.Member, string, error) {
	if m.listAllFn != nil {
		return m.listAllFn(ctx, opts)
	}
	return nil, "", nil
}
func (m *mockMemberStore) ListInScope(ctx context.Context, scopeType domain.ScopeType, scopeID *uuid.UUID, opts port.ListOptions) ([]*domain.Member, string, error) {
	if m.listInScopeFn != nil {
		return m.listInScopeFn(ctx, scopeType, scopeID, opts)
	}
	return nil, "", nil
}
func (m *mockMemberStore) AddRole(ctx context.Context, role *domain.MemberRole) error {
	if m.addRoleFn != nil {
		return m.addRoleFn(ctx, role)
	}
	return nil
}
func (m *mockMemberStore) ReplaceRoleInScope(ctx context.Context, role *domain.MemberRole) error {
	if m.replaceRoleFn != nil {
		return m.replaceRoleFn(ctx, role)
	}
	return nil
}
func (m *mockMemberStore) RemoveRole(ctx context.Context, roleID uuid.UUID) error {
	if m.removeRoleFn != nil {
		return m.removeRoleFn(ctx, roleID)
	}
	return nil
}
func (m *mockMemberStore) RevokeAccessInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error) {
	if m.revokeAccessFn != nil {
		return m.revokeAccessFn(ctx, memberID, scopeType, scopeID)
	}
	return 0, nil
}
func (m *mockMemberStore) GetRoles(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error) {
	if m.getRolesFn != nil {
		return m.getRolesFn(ctx, memberID)
	}
	return nil, nil
}
func (m *mockMemberStore) GetRolesInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
	if m.getRolesInScopeFn != nil {
		return m.getRolesInScopeFn(ctx, memberID, scopeType, scopeID)
	}
	return nil, nil
}
func (m *mockMemberStore) GetRolesByMembers(ctx context.Context, memberIDs []uuid.UUID) (map[uuid.UUID][]*domain.MemberRole, error) {
	if m.getRolesByMembersFn != nil {
		return m.getRolesByMembersFn(ctx, memberIDs)
	}
	result := make(map[uuid.UUID][]*domain.MemberRole, len(memberIDs))
	for _, id := range memberIDs {
		if m.getRolesFn != nil {
			roles, err := m.getRolesFn(ctx, id)
			if err != nil {
				return nil, err
			}
			result[id] = roles
		}
	}
	return result, nil
}

type memberMockTenantStore struct {
	getByCodeFn func(ctx context.Context, code string) (*domain.Tenant, error)
}

func (m *memberMockTenantStore) Create(_ context.Context, _ *domain.Tenant) error { return nil }
func (m *memberMockTenantStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (m *memberMockTenantStore) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	if m.getByCodeFn != nil {
		return m.getByCodeFn(ctx, code)
	}
	return nil, nil
}
func (m *memberMockTenantStore) List(_ context.Context, _ port.ListOptions) ([]*domain.Tenant, string, error) {
	return nil, "", nil
}
func (m *memberMockTenantStore) Update(_ context.Context, _ *domain.Tenant) error { return nil }
func (m *memberMockTenantStore) SoftDelete(_ context.Context, _ uuid.UUID) error  { return nil }
func (m *memberMockTenantStore) Purge(_ context.Context, _ uuid.UUID) error       { return nil }

type memberMockWorkspaceStore struct {
	getByTenantAndCodeFn func(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error)
}

func (m *memberMockWorkspaceStore) Create(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *memberMockWorkspaceStore) CreateLogicalPair(_ context.Context, _ *domain.Workspace, _ *domain.Workspace) error {
	return nil
}
func (m *memberMockWorkspaceStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *memberMockWorkspaceStore) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
	if m.getByTenantAndCodeFn != nil {
		return m.getByTenantAndCodeFn(ctx, tenantID, code, environment)
	}
	return nil, nil
}
func (m *memberMockWorkspaceStore) GetSystemWorkspace(_ context.Context, _ uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
	return nil, nil
}
func (m *memberMockWorkspaceStore) ListByTenant(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
	return nil, "", nil
}
func (m *memberMockWorkspaceStore) UpdateShared(_ context.Context, _ uuid.UUID, _, _, _ string) error {
	return nil
}
func (m *memberMockWorkspaceStore) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *memberMockWorkspaceStore) SoftDeleteLogical(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *memberMockWorkspaceStore) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }

func setupMemberTest(ms port.MemberStore, ts port.TenantStore, ws port.WorkspaceStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if actorID := c.Request().Header.Get("X-Test-Actor-Member-ID"); actorID != "" {
				parsedID, err := uuid.Parse(actorID)
				if err != nil {
					return err
				}
				c.Set(middleware.ContextKeyMember, &domain.Member{ID: parsedID, Email: "actor@senda.dev"})
			}
			if c.Request().Header.Get("X-Test-Actor-Global-Superadmin") == "true" {
				actorID := uuid.MustParse(c.Request().Header.Get("X-Test-Actor-Member-ID"))
				c.Set(middleware.ContextKeyRoles, []*domain.MemberRole{{
					ID:        uuid.New(),
					MemberID:  actorID,
					Role:      domain.RoleSuperadmin,
					ScopeType: domain.ScopeGlobal,
				}})
			}
			return next(c)
		}
	})

	h := handler.NewMemberHandler(ms, ts, ws)
	e.GET("/api/v1/manage/members", h.List)
	e.POST("/api/v1/manage/members", h.Create)
	e.GET("/api/v1/manage/members/:member_id", h.Get)
	e.DELETE("/api/v1/manage/members/:member_id/access", h.RemoveAccess)
	e.PUT("/api/v1/manage/members/:member_id/role", h.ReplaceRole)
	e.POST("/api/v1/manage/members/:member_id/roles", h.AddRole)
	e.DELETE("/api/v1/manage/members/:member_id/roles/:role_id", h.RemoveRole)

	e.GET("/api/v1/manage/tenants/:tenant_code/members", h.ListTenant)
	e.POST("/api/v1/manage/tenants/:tenant_code/members", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/members/:member_id", h.GetTenant)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/members/:member_id/access", h.RemoveAccessTenant)
	e.PUT("/api/v1/manage/tenants/:tenant_code/members/:member_id/role", h.ReplaceRoleTenant)
	e.POST("/api/v1/manage/tenants/:tenant_code/members/:member_id/roles", h.AddRoleTenant)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/members/:member_id/roles/:role_id", h.RemoveRoleTenant)

	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members", h.ListWorkspace)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id", h.GetWorkspace)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/access", h.RemoveAccessWorkspace)
	e.DELETE("/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/access", h.RemoveAccessWorkspace)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/role", h.ReplaceRoleWorkspace)
	e.PUT("/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/role", h.ReplaceRoleWorkspace)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/roles", h.AddRoleWorkspace)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/roles/:role_id", h.RemoveRoleWorkspace)
	return e
}

func TestMemberHandler_Create_Success(t *testing.T) {
	var created *domain.Member
	ms := &mockMemberStore{
		createFn: func(_ context.Context, m *domain.Member) error {
			created = m
			return nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	body := `{"email":"alice@example.com","display_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected member to be created")
	}

	var resp response.MemberWithRolesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Email != "alice@example.com" {
		t.Fatalf("expected email 'alice@example.com', got %q", resp.Email)
	}
	if len(resp.Roles) != 0 {
		t.Fatalf("expected no roles on global create, got %d", len(resp.Roles))
	}
}

func TestMemberHandler_Create_TenantScope_AssignsRole(t *testing.T) {
	tenantID := uuid.New()
	var created *domain.Member
	var addedRole *domain.MemberRole
	ms := &mockMemberStore{
		createFn: func(_ context.Context, m *domain.Member) error {
			created = m
			return nil
		},
		addRoleFn: func(_ context.Context, r *domain.MemberRole) error {
			addedRole = r
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, &memberMockWorkspaceStore{})

	body := `{"email":"bob@example.com","display_name":"Bob","role":"tenant_admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected member to be created")
	}
	if addedRole == nil {
		t.Fatal("expected role to be added")
	}
	if addedRole.Role != domain.RoleTenantAdmin {
		t.Fatalf("expected tenant_admin role, got %s", addedRole.Role)
	}
	if addedRole.ScopeType != domain.ScopeTenant {
		t.Fatalf("expected tenant scope, got %s", addedRole.ScopeType)
	}
	if addedRole.TenantID == nil || *addedRole.TenantID != tenantID {
		t.Fatalf("expected tenant id %s, got %v", tenantID, addedRole.TenantID)
	}
}

func TestMemberHandler_Create_WorkspaceScope_AssignsRole(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	var addedRole *domain.MemberRole
	ms := &mockMemberStore{
		addRoleFn: func(_ context.Context, r *domain.MemberRole) error {
			addedRole = r
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tid)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	body := `{"email":"carol@example.com","role":"workspace_viewer"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/main/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if addedRole == nil {
		t.Fatal("expected role to be added")
	}
	if addedRole.Role != domain.RoleWorkspaceViewer {
		t.Fatalf("expected workspace_viewer role, got %s", addedRole.Role)
	}
	if addedRole.ScopeType != domain.ScopeWorkspace {
		t.Fatalf("expected workspace scope, got %s", addedRole.ScopeType)
	}
	if addedRole.TenantID == nil || *addedRole.TenantID != tenantID {
		t.Fatalf("expected tenant id %s, got %v", tenantID, addedRole.TenantID)
	}
	if addedRole.WorkspaceID == nil || *addedRole.WorkspaceID != workspaceID {
		t.Fatalf("expected workspace id %s, got %v", workspaceID, addedRole.WorkspaceID)
	}
}

func TestMemberHandler_Create_TenantScope_RejectsInvalidRole(t *testing.T) {
	tenantID := uuid.New()
	created := false
	ms := &mockMemberStore{
		createFn: func(_ context.Context, _ *domain.Member) error {
			created = true
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, &memberMockWorkspaceStore{})

	body := `{"email":"bad@example.com","role":"workspace_admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if created {
		t.Fatal("expected member not to be created")
	}
}

func TestMemberHandler_ReplaceRole_Global_CreatesAssignment(t *testing.T) {
	memberID := uuid.New()
	now := time.Now().UTC()
	var replaced *domain.MemberRole
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "admin@senda.dev", CreatedAt: now, UpdatedAt: now}, nil
		},
		replaceRoleFn: func(_ context.Context, role *domain.MemberRole) error {
			replaced = role
			role.CreatedAt = now
			return nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	body := `{"role":"superadmin","scope_type":"global"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/members/"+memberID.String()+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if replaced == nil {
		t.Fatal("expected replace role to be called")
	}
	if replaced.MemberID != memberID {
		t.Fatalf("expected member id %s, got %s", memberID, replaced.MemberID)
	}
	if replaced.Role != domain.RoleSuperadmin || replaced.ScopeType != domain.ScopeGlobal {
		t.Fatalf("unexpected role assignment: %+v", replaced)
	}
}

func TestMemberHandler_ReplaceRole_Global_IdempotentReturnsExistingAssignment(t *testing.T) {
	memberID := uuid.New()
	roleID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour)
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "admin@senda.dev", CreatedAt: createdAt, UpdatedAt: createdAt}, nil
		},
		replaceRoleFn: func(_ context.Context, role *domain.MemberRole) error {
			role.ID = roleID
			role.CreatedAt = createdAt
			return nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	body := `{"role":"superadmin","scope_type":"global"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/members/"+memberID.String()+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.MemberRoleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.ID != roleID.String() {
		t.Fatalf("expected existing role id %s, got %s", roleID, resp.ID)
	}
	if resp.CreatedAt != createdAt.Format("2006-01-02T15:04:05Z07:00") {
		t.Fatalf("expected existing created_at %s, got %s", createdAt.Format("2006-01-02T15:04:05Z07:00"), resp.CreatedAt)
	}
}

func TestMemberHandler_ReplaceRole_Tenant_UsesRouteScope(t *testing.T) {
	memberID := uuid.New()
	tenantID := uuid.New()
	var replaced *domain.MemberRole
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "tenant@senda.dev"}, nil
		},
		replaceRoleFn: func(_ context.Context, role *domain.MemberRole) error {
			replaced = role
			role.CreatedAt = time.Now().UTC()
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, &memberMockWorkspaceStore{})

	body := `{"role":"tenant_admin","scope_type":"tenant"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/members/"+memberID.String()+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if replaced == nil {
		t.Fatal("expected replace role to be called")
	}
	if replaced.ScopeType != domain.ScopeTenant {
		t.Fatalf("expected tenant scope, got %s", replaced.ScopeType)
	}
	if replaced.TenantID == nil || *replaced.TenantID != tenantID {
		t.Fatalf("expected tenant id %s, got %v", tenantID, replaced.TenantID)
	}
}

func TestMemberHandler_ReplaceRole_Workspace_RejectsRoleOutsideRouteScope(t *testing.T) {
	memberID := uuid.New()
	tenantID := uuid.New()
	workspaceID := uuid.New()
	replaced := false
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "workspace@senda.dev"}, nil
		},
		replaceRoleFn: func(_ context.Context, role *domain.MemberRole) error {
			replaced = true
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: workspaceID, TenantID: tid, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	body := `{"role":"tenant_admin","scope_type":"tenant"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/main/members/"+memberID.String()+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if replaced {
		t.Fatal("expected replace role not to be called")
	}
}

func TestMemberHandler_RemoveAccess_Global_Success(t *testing.T) {
	memberID := uuid.New()
	var revokedMemberID uuid.UUID
	var revokedScopeType domain.ScopeType
	var revokedScopeID *uuid.UUID
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			if id != memberID {
				t.Fatalf("expected member id %s, got %s", memberID, id)
			}
			return &domain.Member{ID: memberID, Email: "admin@senda.dev"}, nil
		},
		getRolesInScopeFn: func(_ context.Context, id uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
			if id != memberID {
				t.Fatalf("expected member id %s, got %s", memberID, id)
			}
			if scopeType != domain.ScopeGlobal {
				t.Fatalf("expected global scope, got %s", scopeType)
			}
			if scopeID != nil {
				t.Fatalf("expected nil scope id for global access revoke, got %v", scopeID)
			}
			return []*domain.MemberRole{{ID: uuid.New(), MemberID: memberID, Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal}}, nil
		},
		revokeAccessFn: func(_ context.Context, id uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error) {
			revokedMemberID = id
			revokedScopeType = scopeType
			revokedScopeID = scopeID
			return 1, nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/members/"+memberID.String()+"/access", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if revokedMemberID != memberID {
		t.Fatalf("expected revoke for member %s, got %s", memberID, revokedMemberID)
	}
	if revokedScopeType != domain.ScopeGlobal {
		t.Fatalf("expected global scope revoke, got %s", revokedScopeType)
	}
	if revokedScopeID != nil {
		t.Fatalf("expected nil scope id for global revoke, got %v", revokedScopeID)
	}
}

func TestMemberHandler_RemoveAccess_Global_RejectsSelfRevoke(t *testing.T) {
	memberID := uuid.New()
	revokeCalled := false
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			if id != memberID {
				t.Fatalf("expected member id %s, got %s", memberID, id)
			}
			return &domain.Member{ID: memberID, Email: "admin@senda.dev"}, nil
		},
		revokeAccessFn: func(_ context.Context, _ uuid.UUID, _ domain.ScopeType, _ *uuid.UUID) (int64, error) {
			revokeCalled = true
			return 1, nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/members/"+memberID.String()+"/access", nil)
	req.Header.Set("X-Test-Actor-Member-ID", memberID.String())
	req.Header.Set("X-Test-Actor-Global-Superadmin", "true")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if revokeCalled {
		t.Fatal("expected revoke access NOT to be called on global self-revoke")
	}

	var resp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error.Message != "cannot revoke your own global superadmin access" {
		t.Fatalf("unexpected error message: %q", resp.Error.Message)
	}
}

func TestMemberHandler_RemoveAccess_Global_AllowsOtherMember(t *testing.T) {
	actorID := uuid.New()
	targetID := uuid.New()
	revokeCalled := false
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			if id != targetID {
				t.Fatalf("expected member id %s, got %s", targetID, id)
			}
			return &domain.Member{ID: targetID, Email: "target@senda.dev"}, nil
		},
		revokeAccessFn: func(_ context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error) {
			revokeCalled = true
			if memberID != targetID {
				t.Fatalf("expected revoke for member %s, got %s", targetID, memberID)
			}
			if scopeType != domain.ScopeGlobal {
				t.Fatalf("expected global scope revoke, got %s", scopeType)
			}
			if scopeID != nil {
				t.Fatalf("expected nil global scope id, got %v", scopeID)
			}
			return 1, nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/members/"+targetID.String()+"/access", nil)
	req.Header.Set("X-Test-Actor-Member-ID", actorID.String())
	req.Header.Set("X-Test-Actor-Global-Superadmin", "true")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if !revokeCalled {
		t.Fatal("expected revoke access to be called for another member")
	}
}

func TestMemberHandler_RemoveAccess_Tenant_Success(t *testing.T) {
	memberID := uuid.New()
	tenantID := uuid.New()
	var revokedMemberID uuid.UUID
	var revokedScopeType domain.ScopeType
	var revokedScopeID *uuid.UUID
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			if id != memberID {
				t.Fatalf("expected member id %s, got %s", memberID, id)
			}
			return &domain.Member{ID: memberID, Email: "tenant@senda.dev"}, nil
		},
		getRolesInScopeFn: func(_ context.Context, id uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
			if id != memberID {
				t.Fatalf("expected member id %s, got %s", memberID, id)
			}
			if scopeType != domain.ScopeTenant {
				t.Fatalf("expected tenant scope, got %s", scopeType)
			}
			if scopeID == nil || *scopeID != tenantID {
				t.Fatalf("expected tenant scope id %s, got %v", tenantID, scopeID)
			}
			return []*domain.MemberRole{{ID: uuid.New(), MemberID: memberID, Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeTenant, TenantID: &tenantID}}, nil
		},
		revokeAccessFn: func(_ context.Context, id uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error) {
			revokedMemberID = id
			revokedScopeType = scopeType
			revokedScopeID = scopeID
			return 1, nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, &memberMockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/members/"+memberID.String()+"/access", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if revokedMemberID != memberID {
		t.Fatalf("expected revoke for member %s, got %s", memberID, revokedMemberID)
	}
	if revokedScopeType != domain.ScopeTenant {
		t.Fatalf("expected tenant scope revoke, got %s", revokedScopeType)
	}
	if revokedScopeID == nil || *revokedScopeID != tenantID {
		t.Fatalf("expected tenant scope id %s, got %v", tenantID, revokedScopeID)
	}
}

func TestMemberHandler_RemoveAccess_Workspace_Success(t *testing.T) {
	memberID := uuid.New()
	tenantID := uuid.New()
	workspaceID := uuid.New()
	var revokedMemberID uuid.UUID
	var revokedScopeType domain.ScopeType
	var revokedScopeID *uuid.UUID
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			if id != memberID {
				t.Fatalf("expected member id %s, got %s", memberID, id)
			}
			return &domain.Member{ID: memberID, Email: "workspace@senda.dev"}, nil
		},
		getRolesInScopeFn: func(_ context.Context, id uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
			if id != memberID {
				t.Fatalf("expected member id %s, got %s", memberID, id)
			}
			if scopeType != domain.ScopeWorkspace {
				t.Fatalf("expected workspace scope, got %s", scopeType)
			}
			if scopeID == nil || *scopeID != workspaceID {
				t.Fatalf("expected workspace scope id %s, got %v", workspaceID, scopeID)
			}
			return []*domain.MemberRole{{ID: uuid.New(), MemberID: memberID, Role: domain.RoleWorkspaceEditor, ScopeType: domain.ScopeWorkspace, TenantID: &tenantID, WorkspaceID: &workspaceID}}, nil
		},
		revokeAccessFn: func(_ context.Context, id uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error) {
			revokedMemberID = id
			revokedScopeType = scopeType
			revokedScopeID = scopeID
			return 1, nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tid)
			}
			if environment != domain.EnvironmentTest {
				t.Fatalf("expected test environment, got %s", environment)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code, Environment: domain.EnvironmentTest}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/environments/test/tenants/acme/workspaces/main/members/"+memberID.String()+"/access", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if revokedMemberID != memberID {
		t.Fatalf("expected revoke for member %s, got %s", memberID, revokedMemberID)
	}
	if revokedScopeType != domain.ScopeWorkspace {
		t.Fatalf("expected workspace scope revoke, got %s", revokedScopeType)
	}
	if revokedScopeID == nil || *revokedScopeID != workspaceID {
		t.Fatalf("expected workspace scope id %s, got %v", workspaceID, revokedScopeID)
	}
}

func TestMemberHandler_RemoveAccess_Workspace_IdempotentWhenNoAccessInScope(t *testing.T) {
	memberID := uuid.New()
	tenantID := uuid.New()
	workspaceID := uuid.New()
	revokeCalled := false
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "workspace-no-access@senda.dev"}, nil
		},
		revokeAccessFn: func(_ context.Context, _ uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error) {
			revokeCalled = true
			if scopeType != domain.ScopeWorkspace {
				t.Fatalf("expected workspace scope revoke, got %s", scopeType)
			}
			if scopeID == nil || *scopeID != workspaceID {
				t.Fatalf("expected workspace scope id %s, got %v", workspaceID, scopeID)
			}
			return 0, nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tid)
			}
			if environment != domain.EnvironmentTest {
				t.Fatalf("expected test environment, got %s", environment)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code, Environment: domain.EnvironmentTest}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/environments/test/tenants/acme/workspaces/main/members/"+memberID.String()+"/access", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if !revokeCalled {
		t.Fatal("expected revoke access to be called even when workspace scope has no access")
	}
}

func TestMemberHandler_RemoveAccess_Global_IdempotentWhenNoAccessInScope(t *testing.T) {
	memberID := uuid.New()
	revokeCalled := false
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "missing-access@senda.dev"}, nil
		},
		getRolesInScopeFn: func(_ context.Context, _ uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
			if scopeType != domain.ScopeGlobal {
				t.Fatalf("expected global scope, got %s", scopeType)
			}
			if scopeID != nil {
				t.Fatalf("expected nil scope id, got %v", scopeID)
			}
			return nil, nil
		},
		revokeAccessFn: func(_ context.Context, _ uuid.UUID, _ domain.ScopeType, _ *uuid.UUID) (int64, error) {
			revokeCalled = true
			return 0, nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/members/"+memberID.String()+"/access", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if !revokeCalled {
		t.Fatal("expected revoke access to be called even when scope has no access")
	}
}

func TestMemberHandler_Create_WorkspaceScope_RequiresRole(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	created := false
	ms := &mockMemberStore{
		createFn: func(_ context.Context, _ *domain.Member) error {
			created = true
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tid != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tid)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	body := `{"email":"missing-role@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/main/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if created {
		t.Fatal("expected member not to be created")
	}
}

func TestMemberHandler_Create_InvalidEmail(t *testing.T) {
	e := setupMemberTest(&mockMemberStore{}, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	body := `{"email":"not-an-email"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemberHandler_Create_ExistingEmail_ReusesIdentity(t *testing.T) {
	existingMemberID := uuid.New()
	tenantID := uuid.New()
	workspaceID := uuid.New()
	now := time.Now().UTC()
	displayName := "Existing User"

	createCalled := false
	var addedRole *domain.MemberRole

	ms := &mockMemberStore{
		getByEmailFn: func(_ context.Context, email string) (*domain.Member, error) {
			return &domain.Member{
				ID:          existingMemberID,
				Email:       email,
				DisplayName: &displayName,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
		createFn: func(_ context.Context, _ *domain.Member) error {
			createCalled = true
			return nil
		},
		addRoleFn: func(_ context.Context, r *domain.MemberRole) error {
			addedRole = r
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	body := `{"email":"existing@example.com","display_name":"Existing User","role":"workspace_admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/main/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if createCalled {
		t.Fatal("expected Create NOT to be called for existing member")
	}
	if addedRole == nil {
		t.Fatal("expected role to be added")
	}
	if addedRole.MemberID != existingMemberID {
		t.Fatalf("expected role linked to existing member %s, got %s", existingMemberID, addedRole.MemberID)
	}
	if addedRole.Role != domain.RoleWorkspaceAdmin {
		t.Fatalf("expected workspace_admin role, got %s", addedRole.Role)
	}
}

func TestMemberHandler_Create_GlobalExistingEmail_ReusesIdentity(t *testing.T) {
	existingMemberID := uuid.New()
	now := time.Now().UTC()
	displayName := "Existing User"

	createCalled := false
	ms := &mockMemberStore{
		getByEmailFn: func(_ context.Context, email string) (*domain.Member, error) {
			return &domain.Member{
				ID:          existingMemberID,
				Email:       email,
				DisplayName: &displayName,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
		createFn: func(_ context.Context, _ *domain.Member) error {
			createCalled = true
			return nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	body := `{"email":"existing@example.com","display_name":"Duplicate Attempt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if createCalled {
		t.Fatal("expected Create NOT to be called for existing member")
	}

	var resp response.MemberWithRolesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID != existingMemberID.String() {
		t.Fatalf("expected existing member id %s, got %s", existingMemberID, resp.ID)
	}
	if resp.DisplayName == nil || *resp.DisplayName != displayName {
		t.Fatalf("expected existing display name %q, got %v", displayName, resp.DisplayName)
	}
	if len(resp.Roles) != 0 {
		t.Fatalf("expected no roles on global reuse, got %d", len(resp.Roles))
	}
}

func TestMemberHandler_Get_Success(t *testing.T) {
	memberID := uuid.New()
	now := time.Now().UTC()
	roles := []*domain.MemberRole{
		{ID: uuid.New(), MemberID: memberID, Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal, CreatedAt: now},
	}

	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now}, nil
		},
		getRolesFn: func(_ context.Context, _ uuid.UUID) ([]*domain.MemberRole, error) {
			return roles, nil
		},
	}

	e := setupMemberTest(ms, &memberMockTenantStore{}, &memberMockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/members/"+memberID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp response.MemberWithRolesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", resp.Email)
	}
	if len(resp.Roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(resp.Roles))
	}
}

func TestMemberHandler_TenantList_FiltersRoles(t *testing.T) {
	tenantID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()
	members := []*domain.Member{{ID: memberID, Email: "tenant@example.com", CreatedAt: now, UpdatedAt: now}}
	roles := map[uuid.UUID][]*domain.MemberRole{
		memberID: {
			{ID: uuid.New(), MemberID: memberID, Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeTenant, TenantID: &tenantID, CreatedAt: now},
			{ID: uuid.New(), MemberID: memberID, Role: domain.RoleWorkspaceViewer, ScopeType: domain.ScopeWorkspace, TenantID: &tenantID, WorkspaceID: func() *uuid.UUID { ws := uuid.New(); return &ws }(), CreatedAt: now},
		},
	}

	ms := &mockMemberStore{
		listInScopeFn: func(_ context.Context, scopeType domain.ScopeType, scopeScopeID *uuid.UUID, opts port.ListOptions) ([]*domain.Member, string, error) {
			if scopeType != domain.ScopeTenant {
				t.Fatalf("expected tenant scope, got %s", scopeType)
			}
			if scopeScopeID == nil || *scopeScopeID != tenantID {
				t.Fatalf("expected tenant scope id %s, got %v", tenantID, scopeScopeID)
			}
			return members, "", nil
		},
		getRolesByMembersFn: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]*domain.MemberRole, error) {
			return roles, nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, &memberMockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/members", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.MemberListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 tenant-scoped member, got %d", len(resp.Items))
	}
	if len(resp.Items[0].Roles) != 1 {
		t.Fatalf("expected 1 filtered tenant role, got %d", len(resp.Items[0].Roles))
	}
	if resp.Items[0].Roles[0].Role != string(domain.RoleTenantAdmin) {
		t.Fatalf("expected tenant_admin role, got %s", resp.Items[0].Roles[0].Role)
	}
}

func TestMemberHandler_WorkspaceList_FiltersRoles(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()
	members := []*domain.Member{{ID: memberID, Email: "workspace@example.com", CreatedAt: now, UpdatedAt: now}}
	roles := map[uuid.UUID][]*domain.MemberRole{
		memberID: {
			{ID: uuid.New(), MemberID: memberID, Role: domain.RoleWorkspaceAdmin, ScopeType: domain.ScopeWorkspace, TenantID: &tenantID, WorkspaceID: &workspaceID, CreatedAt: now},
			{ID: uuid.New(), MemberID: memberID, Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeTenant, TenantID: &tenantID, CreatedAt: now},
		},
	}

	ms := &mockMemberStore{
		listInScopeFn: func(_ context.Context, scopeType domain.ScopeType, scopeScopeID *uuid.UUID, opts port.ListOptions) ([]*domain.Member, string, error) {
			if scopeType != domain.ScopeWorkspace {
				t.Fatalf("expected workspace scope, got %s", scopeType)
			}
			if scopeScopeID == nil || *scopeScopeID != workspaceID {
				t.Fatalf("expected workspace scope id %s, got %v", workspaceID, scopeScopeID)
			}
			return members, "", nil
		},
		getRolesByMembersFn: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]*domain.MemberRole, error) {
			return roles, nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tID != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tID)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/main/members", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.MemberListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 workspace-scoped member, got %d", len(resp.Items))
	}
	if len(resp.Items[0].Roles) != 1 {
		t.Fatalf("expected 1 filtered workspace role, got %d", len(resp.Items[0].Roles))
	}
	if resp.Items[0].Roles[0].Role != string(domain.RoleWorkspaceAdmin) {
		t.Fatalf("expected workspace_admin role, got %s", resp.Items[0].Roles[0].Role)
	}
}

func TestMemberHandler_TenantAddRole_RejectsWorkspaceRole(t *testing.T) {
	tenantID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()
	added := false

	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now}, nil
		},
		addRoleFn: func(_ context.Context, _ *domain.MemberRole) error {
			added = true
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, &memberMockWorkspaceStore{})

	body := `{"role":"workspace_admin","scope_type":"workspace"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/members/"+memberID.String()+"/roles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if added {
		t.Fatal("expected role not to be added")
	}
}

func TestMemberHandler_WorkspaceAddRole_Success(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()
	var addedRole *domain.MemberRole

	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now}, nil
		},
		addRoleFn: func(_ context.Context, r *domain.MemberRole) error {
			addedRole = r
			return nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tID != tenantID {
				t.Fatalf("expected tenant id %s, got %s", tenantID, tID)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	body := `{"role":"workspace_viewer","scope_type":"workspace"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/main/members/"+memberID.String()+"/roles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if addedRole == nil {
		t.Fatal("expected role to be added")
	}
	if addedRole.ScopeType != domain.ScopeWorkspace {
		t.Fatalf("expected workspace scope, got %s", addedRole.ScopeType)
	}
	if addedRole.TenantID == nil || *addedRole.TenantID != tenantID {
		t.Fatalf("expected tenant ID %s, got %v", tenantID, addedRole.TenantID)
	}
	if addedRole.WorkspaceID == nil || *addedRole.WorkspaceID != workspaceID {
		t.Fatalf("expected workspace ID %s, got %v", workspaceID, addedRole.WorkspaceID)
	}
}

func TestMemberHandler_WorkspaceGet_404OutsideScope(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()

	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now}, nil
		},
		getRolesInScopeFn: func(_ context.Context, _ uuid.UUID, _ domain.ScopeType, _ *uuid.UUID) ([]*domain.MemberRole, error) {
			return nil, nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/main/members/"+memberID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemberHandler_WorkspaceRemoveRole_RequiresScope(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	memberID := uuid.New()
	roleID := uuid.New()
	now := time.Now().UTC()

	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now}, nil
		},
		getRolesInScopeFn: func(_ context.Context, _ uuid.UUID, _ domain.ScopeType, _ *uuid.UUID) ([]*domain.MemberRole, error) {
			return []*domain.MemberRole{
				{ID: uuid.New(), MemberID: memberID, Role: domain.RoleWorkspaceAdmin, ScopeType: domain.ScopeWorkspace, TenantID: &tenantID, WorkspaceID: &workspaceID, CreatedAt: now},
			}, nil
		},
	}
	ts := &memberMockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			return &domain.Tenant{ID: tenantID, Code: code}, nil
		},
	}
	ws := &memberMockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: code}, nil
		},
	}

	e := setupMemberTest(ms, ts, ws)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/main/members/"+memberID.String()+"/roles/"+roleID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

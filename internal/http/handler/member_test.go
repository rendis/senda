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
	removeRoleFn        func(ctx context.Context, roleID uuid.UUID) error
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
func (m *mockMemberStore) RemoveRole(ctx context.Context, roleID uuid.UUID) error {
	if m.removeRoleFn != nil {
		return m.removeRoleFn(ctx, roleID)
	}
	return nil
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
	getByTenantAndCodeFn func(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error)
}

func (m *memberMockWorkspaceStore) Create(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *memberMockWorkspaceStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *memberMockWorkspaceStore) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
	if m.getByTenantAndCodeFn != nil {
		return m.getByTenantAndCodeFn(ctx, tenantID, code)
	}
	return nil, nil
}
func (m *memberMockWorkspaceStore) GetSystemWorkspace(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *memberMockWorkspaceStore) ListByTenant(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
	return nil, "", nil
}
func (m *memberMockWorkspaceStore) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *memberMockWorkspaceStore) SoftDelete(_ context.Context, _ uuid.UUID) error     { return nil }

func setupMemberTest(ms port.MemberStore, ts port.TenantStore, ws port.WorkspaceStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())

	h := handler.NewMemberHandler(ms, ts, ws)
	e.GET("/api/v1/manage/members", h.List)
	e.POST("/api/v1/manage/members", h.Create)
	e.GET("/api/v1/manage/members/:member_id", h.Get)
	e.POST("/api/v1/manage/members/:member_id/roles", h.AddRole)
	e.DELETE("/api/v1/manage/members/:member_id/roles/:role_id", h.RemoveRole)

	e.GET("/api/v1/manage/tenants/:tenant_code/members", h.ListTenant)
	e.POST("/api/v1/manage/tenants/:tenant_code/members", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/members/:member_id", h.GetTenant)
	e.POST("/api/v1/manage/tenants/:tenant_code/members/:member_id/roles", h.AddRoleTenant)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/members/:member_id/roles/:role_id", h.RemoveRoleTenant)

	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members", h.ListWorkspace)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id", h.GetWorkspace)
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
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string) (*domain.Workspace, error) {
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
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string) (*domain.Workspace, error) {
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
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string) (*domain.Workspace, error) {
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
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string) (*domain.Workspace, error) {
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
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string) (*domain.Workspace, error) {
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
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string) (*domain.Workspace, error) {
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
		getByTenantAndCodeFn: func(_ context.Context, tID uuid.UUID, code string) (*domain.Workspace, error) {
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

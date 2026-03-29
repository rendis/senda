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

// --- Mock MemberStore ---

type mockMemberStore struct {
	createFn            func(ctx context.Context, m *domain.Member) error
	getByEmailFn        func(ctx context.Context, email string) (*domain.Member, error)
	getByIDFn           func(ctx context.Context, id uuid.UUID) (*domain.Member, error)
	countAllFn          func(ctx context.Context) (int64, error)
	listAllFn           func(ctx context.Context, opts port.ListOptions) ([]*domain.Member, string, error)
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
	// Fallback: aggregate from per-member getRolesFn if available.
	if m.getRolesFn != nil {
		result := make(map[uuid.UUID][]*domain.MemberRole, len(memberIDs))
		for _, id := range memberIDs {
			roles, err := m.getRolesFn(ctx, id)
			if err != nil {
				return nil, err
			}
			result[id] = roles
		}
		return result, nil
	}
	return make(map[uuid.UUID][]*domain.MemberRole), nil
}

func setupMemberTest(ms port.MemberStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())

	h := handler.NewMemberHandler(ms)
	e.GET("/api/v1/manage/members", h.List)
	e.POST("/api/v1/manage/members", h.Create)
	e.GET("/api/v1/manage/members/:member_id", h.Get)
	e.POST("/api/v1/manage/members/:member_id/roles", h.AddRole)
	e.DELETE("/api/v1/manage/members/:member_id/roles/:role_id", h.RemoveRole)
	return e
}

// --- Tests ---

func TestMemberHandler_Create_Success(t *testing.T) {
	var created *domain.Member
	ms := &mockMemberStore{
		createFn: func(_ context.Context, m *domain.Member) error {
			created = m
			return nil
		},
	}

	e := setupMemberTest(ms)

	body := `{"email":"alice@example.com","display_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.MemberResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Email != "alice@example.com" {
		t.Fatalf("expected email 'alice@example.com', got %q", resp.Email)
	}
	if created == nil {
		t.Fatal("expected member to be created")
	}
}

func TestMemberHandler_Create_InvalidEmail(t *testing.T) {
	e := setupMemberTest(&mockMemberStore{})

	body := `{"email":"not-an-email"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemberHandler_Create_MissingEmail(t *testing.T) {
	e := setupMemberTest(&mockMemberStore{})

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemberHandler_Create_Conflict(t *testing.T) {
	ms := &mockMemberStore{
		createFn: func(_ context.Context, _ *domain.Member) error {
			return domain.ErrConflict
		},
	}

	e := setupMemberTest(ms)

	body := `{"email":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
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
			return &domain.Member{
				ID: id, Email: "alice@example.com",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		getRolesFn: func(_ context.Context, _ uuid.UUID) ([]*domain.MemberRole, error) {
			return roles, nil
		},
	}

	e := setupMemberTest(ms)

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
		t.Fatalf("expected email 'alice@example.com', got %q", resp.Email)
	}
	if len(resp.Roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(resp.Roles))
	}
}

func TestMemberHandler_Get_InvalidUUID(t *testing.T) {
	e := setupMemberTest(&mockMemberStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/members/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemberHandler_Me_Success(t *testing.T) {
	now := time.Now().UTC()
	memberID := uuid.New()
	tenantID := uuid.New()
	workspaceID := uuid.New()

	h := handler.NewMemberHandler(&mockMemberStore{})
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	req := httptest.NewRequest(http.MethodGet, "/api/v1/members/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	member := &domain.Member{
		ID:        memberID,
		Email:     "alice@example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}
	roles := []*domain.MemberRole{
		{
			ID:          uuid.New(),
			MemberID:    memberID,
			Role:        domain.RoleTenantAdmin,
			ScopeType:   domain.ScopeTenant,
			TenantID:    &tenantID,
			WorkspaceID: &workspaceID,
			CreatedAt:   now,
		},
	}

	c.Set(middleware.ContextKeyMember, member)
	c.Set(middleware.ContextKeyRoles, roles)

	if err := h.Me(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
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
	if resp.Roles[0].Role != string(domain.RoleTenantAdmin) {
		t.Fatalf("expected role tenant_admin, got %s", resp.Roles[0].Role)
	}
}

func TestMemberHandler_Me_MissingMemberContext(t *testing.T) {
	h := handler.NewMemberHandler(&mockMemberStore{})
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	req := httptest.NewRequest(http.MethodGet, "/api/v1/members/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Me(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemberHandler_AddRole_Success(t *testing.T) {
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

	e := setupMemberTest(ms)

	body := `{"role":"superadmin","scope_type":"global"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members/"+memberID.String()+"/roles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if addedRole == nil {
		t.Fatal("expected role to be added")
	}
	if addedRole.Role != domain.RoleSuperadmin {
		t.Fatalf("expected role superadmin, got %q", addedRole.Role)
	}
}

func TestMemberHandler_AddRole_InvalidRole(t *testing.T) {
	memberID := uuid.New()
	now := time.Now().UTC()
	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now}, nil
		},
	}

	e := setupMemberTest(ms)

	body := `{"role":"megaboss","scope_type":"global"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/members/"+memberID.String()+"/roles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMemberHandler_List_Success(t *testing.T) {
	now := time.Now().UTC()
	member1 := uuid.New()
	member2 := uuid.New()

	members := []*domain.Member{
		{ID: member1, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now},
		{ID: member2, Email: "bob@example.com", CreatedAt: now, UpdatedAt: now},
	}
	rolesMap := map[uuid.UUID][]*domain.MemberRole{
		member1: {{ID: uuid.New(), MemberID: member1, Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal, CreatedAt: now}},
		member2: {},
	}

	ms := &mockMemberStore{
		listAllFn: func(_ context.Context, opts port.ListOptions) ([]*domain.Member, string, error) {
			if opts.Limit != 25 {
				t.Fatalf("expected default limit 25, got %d", opts.Limit)
			}
			return members, "next-cursor-token", nil
		},
		getRolesFn: func(_ context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error) {
			return rolesMap[memberID], nil
		},
	}

	e := setupMemberTest(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/members", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.MemberListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Items[0].Email != "alice@example.com" {
		t.Fatalf("expected first member email 'alice@example.com', got %q", resp.Items[0].Email)
	}
	if len(resp.Items[0].Roles) != 1 {
		t.Fatalf("expected 1 role for alice, got %d", len(resp.Items[0].Roles))
	}
	if len(resp.Items[1].Roles) != 0 {
		t.Fatalf("expected 0 roles for bob, got %d", len(resp.Items[1].Roles))
	}
	if resp.NextCursor != "next-cursor-token" {
		t.Fatalf("expected next_cursor 'next-cursor-token', got %q", resp.NextCursor)
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
}

func TestMemberHandler_RemoveRole_Success(t *testing.T) {
	memberID := uuid.New()
	roleID := uuid.New()
	now := time.Now().UTC()
	var removedID uuid.UUID

	ms := &mockMemberStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Member, error) {
			return &domain.Member{ID: id, Email: "alice@example.com", CreatedAt: now, UpdatedAt: now}, nil
		},
		removeRoleFn: func(_ context.Context, id uuid.UUID) error {
			removedID = id
			return nil
		},
	}

	e := setupMemberTest(ms)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/members/"+memberID.String()+"/roles/"+roleID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if removedID != roleID {
		t.Fatalf("expected removed role ID %s, got %s", roleID, removedID)
	}
}

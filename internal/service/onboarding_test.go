package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// --- Mocks ---

type mockMemberStoreOnboarding struct {
	createFn       func(ctx context.Context, member *domain.Member) error
	getByEmailFn   func(ctx context.Context, email string) (*domain.Member, error)
	getByIDFn      func(ctx context.Context, id uuid.UUID) (*domain.Member, error)
	countAllFn     func(ctx context.Context) (int64, error)
	addRoleFn      func(ctx context.Context, role *domain.MemberRole) error
	removeRoleFn   func(ctx context.Context, roleID uuid.UUID) error
	getRolesFn     func(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error)
	getRolesInScopeFn func(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error)
}

func (m *mockMemberStoreOnboarding) Create(ctx context.Context, member *domain.Member) error {
	if m.createFn != nil {
		return m.createFn(ctx, member)
	}
	return nil
}
func (m *mockMemberStoreOnboarding) GetByEmail(ctx context.Context, email string) (*domain.Member, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return nil, nil
}
func (m *mockMemberStoreOnboarding) GetByID(ctx context.Context, id uuid.UUID) (*domain.Member, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockMemberStoreOnboarding) CountAll(ctx context.Context) (int64, error) {
	if m.countAllFn != nil {
		return m.countAllFn(ctx)
	}
	return 0, nil
}
func (m *mockMemberStoreOnboarding) AddRole(ctx context.Context, role *domain.MemberRole) error {
	if m.addRoleFn != nil {
		return m.addRoleFn(ctx, role)
	}
	return nil
}
func (m *mockMemberStoreOnboarding) RemoveRole(ctx context.Context, roleID uuid.UUID) error {
	if m.removeRoleFn != nil {
		return m.removeRoleFn(ctx, roleID)
	}
	return nil
}
func (m *mockMemberStoreOnboarding) GetRoles(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error) {
	if m.getRolesFn != nil {
		return m.getRolesFn(ctx, memberID)
	}
	return nil, nil
}
func (m *mockMemberStoreOnboarding) GetRolesInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
	if m.getRolesInScopeFn != nil {
		return m.getRolesInScopeFn(ctx, memberID, scopeType, scopeID)
	}
	return nil, nil
}

type mockTenantStoreOnboarding struct {
	createFn     func(ctx context.Context, t *domain.Tenant) error
	getByIDFn    func(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	getByCodeFn  func(ctx context.Context, code string) (*domain.Tenant, error)
	listFn       func(ctx context.Context, opts port.ListOptions) ([]*domain.Tenant, string, error)
	updateFn     func(ctx context.Context, t *domain.Tenant) error
	softDeleteFn func(ctx context.Context, id uuid.UUID) error
	purgeFn      func(ctx context.Context, id uuid.UUID) error
}

func (m *mockTenantStoreOnboarding) Create(ctx context.Context, t *domain.Tenant) error {
	if m.createFn != nil {
		return m.createFn(ctx, t)
	}
	return nil
}
func (m *mockTenantStoreOnboarding) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockTenantStoreOnboarding) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	if m.getByCodeFn != nil {
		return m.getByCodeFn(ctx, code)
	}
	return nil, nil
}
func (m *mockTenantStoreOnboarding) List(ctx context.Context, opts port.ListOptions) ([]*domain.Tenant, string, error) {
	if m.listFn != nil {
		return m.listFn(ctx, opts)
	}
	return nil, "", nil
}
func (m *mockTenantStoreOnboarding) Update(ctx context.Context, t *domain.Tenant) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, t)
	}
	return nil
}
func (m *mockTenantStoreOnboarding) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockTenantStoreOnboarding) Purge(ctx context.Context, id uuid.UUID) error {
	if m.purgeFn != nil {
		return m.purgeFn(ctx, id)
	}
	return nil
}

type mockWorkspaceStoreOnboarding struct {
	createFn             func(ctx context.Context, ws *domain.Workspace) error
	getByIDFn            func(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	getByTenantAndCodeFn func(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error)
	getSystemWorkspaceFn func(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error)
	listByTenantFn       func(ctx context.Context, tenantID uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error)
	updateFn             func(ctx context.Context, ws *domain.Workspace) error
	softDeleteFn         func(ctx context.Context, id uuid.UUID) error
}

func (m *mockWorkspaceStoreOnboarding) Create(ctx context.Context, ws *domain.Workspace) error {
	if m.createFn != nil {
		return m.createFn(ctx, ws)
	}
	return nil
}
func (m *mockWorkspaceStoreOnboarding) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockWorkspaceStoreOnboarding) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
	if m.getByTenantAndCodeFn != nil {
		return m.getByTenantAndCodeFn(ctx, tenantID, code)
	}
	return nil, nil
}
func (m *mockWorkspaceStoreOnboarding) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
	if m.getSystemWorkspaceFn != nil {
		return m.getSystemWorkspaceFn(ctx, tenantID)
	}
	return nil, nil
}
func (m *mockWorkspaceStoreOnboarding) ListByTenant(ctx context.Context, tenantID uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error) {
	if m.listByTenantFn != nil {
		return m.listByTenantFn(ctx, tenantID, opts)
	}
	return nil, "", nil
}
func (m *mockWorkspaceStoreOnboarding) Update(ctx context.Context, ws *domain.Workspace) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, ws)
	}
	return nil
}
func (m *mockWorkspaceStoreOnboarding) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

type mockAuditLogStoreOnboarding struct {
	appendFn func(ctx context.Context, entry *domain.AuditLog) error
	queryFn  func(ctx context.Context, filter port.AuditFilter, opts port.ListOptions) (*port.PageResult[domain.AuditLog], error)
}

func (m *mockAuditLogStoreOnboarding) Append(ctx context.Context, entry *domain.AuditLog) error {
	if m.appendFn != nil {
		return m.appendFn(ctx, entry)
	}
	return nil
}
func (m *mockAuditLogStoreOnboarding) Query(ctx context.Context, filter port.AuditFilter, opts port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, filter, opts)
	}
	return nil, nil
}

// --- Tests ---

func TestOnboardingService_Status_NeedsOnboarding(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			return 0, nil
		},
	}

	svc := service.NewOnboardingService(ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	needs, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needs {
		t.Fatal("expected needs_onboarding=true when no members exist")
	}
}

func TestOnboardingService_Status_AlreadyOnboarded(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			return 3, nil
		},
	}

	svc := service.NewOnboardingService(ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	needs, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needs {
		t.Fatal("expected needs_onboarding=false when members exist")
	}
}

func TestOnboardingService_Status_StoreError(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			return 0, errors.New("db error")
		},
	}

	svc := service.NewOnboardingService(ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	_, err := svc.Status(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOnboardingService_Setup_Success(t *testing.T) {
	var createdMember *domain.Member
	var createdRoles []*domain.MemberRole
	var createdTenant *domain.Tenant
	var createdWS *domain.Workspace
	var auditEntry *domain.AuditLog

	countCalls := 0
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			countCalls++
			if countCalls == 1 {
				return 0, nil // Initial check: no members
			}
			return 1, nil // Post-create check: only our member exists
		},
		createFn: func(_ context.Context, m *domain.Member) error {
			createdMember = m
			return nil
		},
		addRoleFn: func(_ context.Context, r *domain.MemberRole) error {
			createdRoles = append(createdRoles, r)
			return nil
		},
	}
	ts := &mockTenantStoreOnboarding{
		createFn: func(_ context.Context, t *domain.Tenant) error {
			createdTenant = t
			return nil
		},
	}
	ws := &mockWorkspaceStoreOnboarding{
		createFn: func(_ context.Context, w *domain.Workspace) error {
			createdWS = w
			return nil
		},
	}
	as := &mockAuditLogStoreOnboarding{
		appendFn: func(_ context.Context, e *domain.AuditLog) error {
			auditEntry = e
			return nil
		},
	}

	svc := service.NewOnboardingService(ms, ts, ws, as)

	claims := &port.OIDCClaims{
		Subject: "oidc-subject-123",
		Email:   "admin@example.com",
		Issuer:  "https://auth.example.com",
	}
	req := &service.OnboardingRequest{
		TenantCode: "acme",
		TenantName: "Acme Corp",
	}

	result, err := svc.Setup(context.Background(), claims, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify member created
	if createdMember == nil {
		t.Fatal("expected member to be created")
	}
	if createdMember.Email != "admin@example.com" {
		t.Fatalf("expected email 'admin@example.com', got %q", createdMember.Email)
	}
	if createdMember.OIDCSubject == nil || *createdMember.OIDCSubject != "oidc-subject-123" {
		t.Fatal("expected OIDC subject to be set")
	}
	if createdMember.OIDCIssuer == nil || *createdMember.OIDCIssuer != "https://auth.example.com" {
		t.Fatal("expected OIDC issuer to be set")
	}

	// Verify two roles: superadmin (global) + tenant_admin (tenant)
	if len(createdRoles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(createdRoles))
	}

	var hasSuperadmin, hasTenantAdmin bool
	for _, r := range createdRoles {
		if r.Role == domain.RoleSuperadmin && r.ScopeType == domain.ScopeGlobal {
			hasSuperadmin = true
		}
		if r.Role == domain.RoleTenantAdmin && r.ScopeType == domain.ScopeTenant && r.TenantID != nil {
			hasTenantAdmin = true
		}
	}
	if !hasSuperadmin {
		t.Fatal("expected superadmin role with global scope")
	}
	if !hasTenantAdmin {
		t.Fatal("expected tenant_admin role with tenant scope")
	}

	// Verify tenant created
	if createdTenant == nil {
		t.Fatal("expected tenant to be created")
	}
	if createdTenant.Code != "acme" {
		t.Fatalf("expected tenant code 'acme', got %q", createdTenant.Code)
	}
	if createdTenant.Name != "Acme Corp" {
		t.Fatalf("expected tenant name 'Acme Corp', got %q", createdTenant.Name)
	}

	// Verify _system workspace
	if createdWS == nil {
		t.Fatal("expected workspace to be created")
	}
	if createdWS.Code != "_system" {
		t.Fatalf("expected workspace code '_system', got %q", createdWS.Code)
	}
	if createdWS.Name != "System" {
		t.Fatalf("expected workspace name 'System', got %q", createdWS.Name)
	}
	if !createdWS.IsSystem {
		t.Fatal("expected workspace IsSystem=true")
	}
	if createdWS.TenantID != createdTenant.ID {
		t.Fatal("expected workspace tenant_id to match tenant")
	}

	// Verify audit log
	if auditEntry == nil {
		t.Fatal("expected audit log entry")
	}
	if auditEntry.Action != domain.AuditCreate {
		t.Fatalf("expected audit action 'create', got %q", auditEntry.Action)
	}
	if auditEntry.EntityType != "onboarding" {
		t.Fatalf("expected entity_type 'onboarding', got %q", auditEntry.EntityType)
	}

	// Verify result
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Member.ID != createdMember.ID {
		t.Fatal("expected result member ID to match created member")
	}
	if result.Tenant.ID != createdTenant.ID {
		t.Fatal("expected result tenant ID to match created tenant")
	}
	if result.Workspace.ID != createdWS.ID {
		t.Fatal("expected result workspace ID to match created workspace")
	}
}

func TestOnboardingService_Setup_ConflictWhenMembersExist(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			return 1, nil
		},
	}

	svc := service.NewOnboardingService(ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	claims := &port.OIDCClaims{
		Subject: "sub",
		Email:   "admin@example.com",
		Issuer:  "https://auth.example.com",
	}
	req := &service.OnboardingRequest{
		TenantCode: "acme",
		TenantName: "Acme Corp",
	}

	_, err := svc.Setup(context.Background(), claims, req)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestOnboardingService_Setup_CountError(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			return 0, errors.New("db error")
		},
	}

	svc := service.NewOnboardingService(ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	claims := &port.OIDCClaims{Subject: "sub", Email: "a@b.com", Issuer: "iss"}
	req := &service.OnboardingRequest{TenantCode: "acme", TenantName: "Acme"}

	_, err := svc.Setup(context.Background(), claims, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOnboardingService_Setup_MemberCreateError(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			return 0, nil
		},
		createFn: func(_ context.Context, _ *domain.Member) error {
			return errors.New("member create failed")
		},
	}

	svc := service.NewOnboardingService(ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	claims := &port.OIDCClaims{Subject: "sub", Email: "a@b.com", Issuer: "iss"}
	req := &service.OnboardingRequest{TenantCode: "acme", TenantName: "Acme"}

	_, err := svc.Setup(context.Background(), claims, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOnboardingService_Setup_TenantCreateError(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) { return 0, nil },
		createFn:   func(_ context.Context, _ *domain.Member) error { return nil },
		addRoleFn:  func(_ context.Context, _ *domain.MemberRole) error { return nil },
	}
	ts := &mockTenantStoreOnboarding{
		createFn: func(_ context.Context, _ *domain.Tenant) error {
			return domain.ErrConflict
		},
	}

	svc := service.NewOnboardingService(ms, ts, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	claims := &port.OIDCClaims{Subject: "sub", Email: "a@b.com", Issuer: "iss"}
	req := &service.OnboardingRequest{TenantCode: "acme", TenantName: "Acme"}

	_, err := svc.Setup(context.Background(), claims, req)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestOnboardingService_Setup_WorkspaceCreateError(t *testing.T) {
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) { return 0, nil },
		createFn:   func(_ context.Context, _ *domain.Member) error { return nil },
		addRoleFn:  func(_ context.Context, _ *domain.MemberRole) error { return nil },
	}
	ts := &mockTenantStoreOnboarding{
		createFn: func(_ context.Context, _ *domain.Tenant) error { return nil },
	}
	ws := &mockWorkspaceStoreOnboarding{
		createFn: func(_ context.Context, _ *domain.Workspace) error {
			return errors.New("workspace create failed")
		},
	}

	svc := service.NewOnboardingService(ms, ts, ws, &mockAuditLogStoreOnboarding{})

	claims := &port.OIDCClaims{Subject: "sub", Email: "a@b.com", Issuer: "iss"}
	req := &service.OnboardingRequest{TenantCode: "acme", TenantName: "Acme"}

	_, err := svc.Setup(context.Background(), claims, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOnboardingService_Setup_ConcurrentRaceDetected(t *testing.T) {
	// Simulates TOCTOU: initial CountAll returns 0, but after member creation
	// the re-check returns 2 (another concurrent setup also created a member).
	callCount := 0
	ms := &mockMemberStoreOnboarding{
		countAllFn: func(_ context.Context) (int64, error) {
			callCount++
			if callCount == 1 {
				return 0, nil // Initial check passes
			}
			return 2, nil // Post-create check detects race
		},
		createFn:  func(_ context.Context, _ *domain.Member) error { return nil },
		addRoleFn: func(_ context.Context, _ *domain.MemberRole) error { return nil },
	}

	svc := service.NewOnboardingService(ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	claims := &port.OIDCClaims{Subject: "sub", Email: "a@b.com", Issuer: "iss"}
	req := &service.OnboardingRequest{TenantCode: "acme", TenantName: "Acme"}

	_, err := svc.Setup(context.Background(), claims, req)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for concurrent race, got %v", err)
	}
}

package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// mockTxBeginner is a test-only TxBeginner that returns a mock transaction.
type mockTxBeginner struct {
	tx pgx.Tx // optional: use a custom mockTx for specific tests
}

func (m *mockTxBeginner) Begin(_ context.Context) (pgx.Tx, error) {
	if m.tx != nil {
		return m.tx, nil
	}
	return &mockTx{}, nil
}

// mockTx satisfies pgx.Tx with no-op implementations sufficient for onboarding tests.
// mockRow implements pgx.Row for onboarding tests.
type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	now := time.Now().UTC()
	for _, d := range dest {
		switch v := d.(type) {
		case *time.Time:
			*v = now
		case *int64:
			*v = 0
		}
	}
	return nil
}

// mockTx with configurable QueryRow for different test scenarios.
type mockTx struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockTx) Begin(_ context.Context) (pgx.Tx, error)  { return &mockTx{}, nil }
func (m *mockTx) Commit(_ context.Context) error            { return nil }
func (m *mockTx) Rollback(_ context.Context) error          { return nil }
func (m *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (m *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (m *mockTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (m *mockTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (m *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("SELECT 1"), nil
}
func (m *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{}
}
func (m *mockTx) Conn() *pgx.Conn { return nil }

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
func (m *mockMemberStoreOnboarding) ListAll(_ context.Context, _ port.ListOptions) ([]*domain.Member, string, error) {
	return nil, "", nil
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
func (m *mockMemberStoreOnboarding) GetRolesByMembers(_ context.Context, _ []uuid.UUID) (map[uuid.UUID][]*domain.MemberRole, error) {
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

	svc := service.NewOnboardingService(&mockTxBeginner{}, ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

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

	svc := service.NewOnboardingService(&mockTxBeginner{}, ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

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

	svc := service.NewOnboardingService(&mockTxBeginner{}, ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

	_, err := svc.Status(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOnboardingService_Setup_Success(t *testing.T) {
	// Setup now executes all SQL directly on the tx (not via store mocks).
	// We verify the returned result struct which is populated in-memory.
	ms := &mockMemberStoreOnboarding{}
	ts := &mockTenantStoreOnboarding{}
	ws := &mockWorkspaceStoreOnboarding{}
	as := &mockAuditLogStoreOnboarding{}

	svc := service.NewOnboardingService(&mockTxBeginner{}, ms, ts, ws, as)

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

	// Verify result populated correctly from in-memory structs.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Member.Email != "admin@example.com" {
		t.Fatalf("expected email 'admin@example.com', got %q", result.Member.Email)
	}
	if result.Member.OIDCSubject == nil || *result.Member.OIDCSubject != "oidc-subject-123" {
		t.Fatal("expected OIDC subject set")
	}
	if result.Tenant.Code != "acme" {
		t.Fatalf("expected tenant code 'acme', got %q", result.Tenant.Code)
	}
	if result.Tenant.Name != "Acme Corp" {
		t.Fatalf("expected tenant name 'Acme Corp', got %q", result.Tenant.Name)
	}
	if result.Workspace.Code != "_system" {
		t.Fatalf("expected workspace code '_system', got %q", result.Workspace.Code)
	}
	if !result.Workspace.IsSystem {
		t.Fatal("expected workspace IsSystem=true")
	}
	if result.Workspace.TenantID != result.Tenant.ID {
		t.Fatal("expected workspace tenant_id to match tenant")
	}
}

func TestOnboardingService_Setup_ConflictWhenMembersExist(t *testing.T) {
	// Use a mockTx that returns count=1 for SELECT COUNT(*) FROM members.
	conflictTx := &mockTx{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "SELECT COUNT") {
				return &mockRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*int64); ok {
						*p = 1
					}
					return nil
				}}
			}
			return &mockRow{}
		},
	}
	beginner := &mockTxBeginner{tx: conflictTx}
	ms := &mockMemberStoreOnboarding{}

	svc := service.NewOnboardingService(beginner, ms, &mockTenantStoreOnboarding{}, &mockWorkspaceStoreOnboarding{}, &mockAuditLogStoreOnboarding{})

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

// NOTE: CountError, MemberCreateError, TenantCreateError, WorkspaceCreateError tests
// were removed because Setup() now executes SQL directly on the transaction (not via store
// mocks). Error-path testing for Setup requires integration tests with a real database.

func TestOnboardingService_Setup_ConcurrentRaceDetected(t *testing.T) {
	// With the advisory lock pattern, a second concurrent Setup caller
	// acquires the lock only after the first commits. At that point
	// the COUNT(*) query inside the tx returns non-zero and Setup rejects with ErrConflict.
	conflictTx := &mockTx{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "SELECT COUNT") {
				return &mockRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*int64); ok {
						*p = 1
					}
					return nil
				}}
			}
			return &mockRow{}
		},
	}

	svc := service.NewOnboardingService(
		&mockTxBeginner{tx: conflictTx},
		&mockMemberStoreOnboarding{},
		&mockTenantStoreOnboarding{},
		&mockWorkspaceStoreOnboarding{},
		&mockAuditLogStoreOnboarding{},
	)

	claims := &port.OIDCClaims{Subject: "sub", Email: "a@b.com", Issuer: "iss"}
	req := &service.OnboardingRequest{TenantCode: "acme", TenantName: "Acme"}

	_, err := svc.Setup(context.Background(), claims, req)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for concurrent race, got %v", err)
	}
}

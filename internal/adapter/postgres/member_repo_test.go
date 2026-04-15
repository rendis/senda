//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

func createTestMember(ctx context.Context, t *testing.T, repo *pgadapter.MemberRepo) *domain.Member {
	t.Helper()
	name := "Test User"
	member := &domain.Member{
		ID:          uuid.New(),
		Email:       uuid.New().String()[:8] + "@test.com",
		DisplayName: &name,
	}
	if err := repo.Create(ctx, member); err != nil {
		t.Fatalf("creating test member: %v", err)
	}
	return member
}

func TestMemberRepo_Create(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	name := "John Doe"
	member := &domain.Member{
		ID:          uuid.New(),
		Email:       "john@example.com",
		DisplayName: &name,
	}

	if err := repo.Create(ctx, member); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if member.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if member.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestMemberRepo_Create_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	email := "dup@example.com"
	m1 := &domain.Member{ID: uuid.New(), Email: email}
	if err := repo.Create(ctx, m1); err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	m2 := &domain.Member{ID: uuid.New(), Email: email}
	err := repo.Create(ctx, m2)
	if err == nil {
		t.Fatal("expected conflict error for duplicate email")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestMemberRepo_GetByEmail(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, repo)

	got, err := repo.GetByEmail(ctx, member.Email)
	if err != nil {
		t.Fatalf("GetByEmail() error: %v", err)
	}
	if got.ID != member.ID {
		t.Errorf("want ID %s, got %s", member.ID, got.ID)
	}
	if got.DisplayName == nil || *got.DisplayName != *member.DisplayName {
		t.Errorf("want DisplayName %v, got %v", member.DisplayName, got.DisplayName)
	}
}

func TestMemberRepo_GetByEmail_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestMemberRepo_GetByOIDCIdentity(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	displayName := "OIDC Member"
	subject := "subject-123"
	issuer := "https://auth.example.com"
	member := &domain.Member{
		ID:          uuid.New(),
		Email:       "oidc-member@example.com",
		DisplayName: &displayName,
		OIDCSubject: &subject,
		OIDCIssuer:  &issuer,
	}
	if err := repo.Create(ctx, member); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByOIDCIdentity(ctx, issuer, subject)
	if err != nil {
		t.Fatalf("GetByOIDCIdentity() error: %v", err)
	}
	if got.ID != member.ID {
		t.Errorf("want ID %s, got %s", member.ID, got.ID)
	}
	if got.Email != member.Email {
		t.Errorf("want email %s, got %s", member.Email, got.Email)
	}
}

func TestMemberRepo_GetByOIDCIdentity_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	_, err := repo.GetByOIDCIdentity(ctx, "https://auth.example.com", "missing-subject")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestMemberRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, repo)

	got, err := repo.GetByID(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Email != member.Email {
		t.Errorf("want Email %s, got %s", member.Email, got.Email)
	}
}

func TestMemberRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	_, err := repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestMemberRepo_CountAll(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	// Initially 0
	count, err := repo.CountAll(ctx)
	if err != nil {
		t.Fatalf("CountAll() error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	createTestMember(ctx, t, repo)
	createTestMember(ctx, t, repo)

	count, err = repo.CountAll(ctx)
	if err != nil {
		t.Fatalf("CountAll() error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestMemberRepo_AddRole_Global(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, memberRepo)

	role := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleSuperadmin,
		ScopeType: domain.ScopeGlobal,
	}

	if err := memberRepo.AddRole(ctx, role); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	if role.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestMemberRepo_AddRole_Tenant(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	member := createTestMember(ctx, t, memberRepo)

	role := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleTenantAdmin,
		ScopeType: domain.ScopeTenant,
		TenantID:  &tenant.ID,
		CreatedBy: &member.ID,
	}

	if err := memberRepo.AddRole(ctx, role); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	if role.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestMemberRepo_AddRole_Workspace(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}

	member := createTestMember(ctx, t, memberRepo)

	role := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceEditor,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &ws.ID,
		CreatedBy:   &member.ID,
	}

	if err := memberRepo.AddRole(ctx, role); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}
}

func TestMemberRepo_AddRole_DuplicateConflict(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, memberRepo)

	role := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal,
	}
	if err := memberRepo.AddRole(ctx, role); err != nil {
		t.Fatalf("first AddRole() error: %v", err)
	}

	dup := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal,
	}
	err := memberRepo.AddRole(ctx, dup)
	if err == nil {
		t.Fatal("expected conflict error for duplicate role")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestMemberRepo_AddRole_InvalidCombination(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, memberRepo)

	// tenant_admin with scope_type=global should violate check constraint
	role := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeGlobal,
	}
	err := memberRepo.AddRole(ctx, role)
	if err == nil {
		t.Fatal("expected validation error for invalid role/scope")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 422 {
		t.Errorf("expected 422 Validation, got: %v", err)
	}
}

func TestMemberRepo_ReplaceRoleInScope_CreatesAssignmentWhenMissing(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, memberRepo)
	role := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleSuperadmin,
		ScopeType: domain.ScopeGlobal,
	}

	if err := memberRepo.ReplaceRoleInScope(ctx, role); err != nil {
		t.Fatalf("ReplaceRoleInScope() error: %v", err)
	}
	if role.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}

	roles, err := memberRepo.GetRolesInScope(ctx, member.ID, domain.ScopeGlobal, nil)
	if err != nil {
		t.Fatalf("GetRolesInScope() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(roles))
	}
	if roles[0].Role != domain.RoleSuperadmin {
		t.Fatalf("expected superadmin, got %s", roles[0].Role)
	}
}

func TestMemberRepo_ReplaceRoleInScope_ReplacesExistingWorkspaceRole(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	workspace := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Code:        "ws-" + uuid.New().String()[:8],
		Name:        "Test WS",
		Environment: domain.EnvironmentProd,
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	original := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceViewer,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.AddRole(ctx, original); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	replacement := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceAdmin,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.ReplaceRoleInScope(ctx, replacement); err != nil {
		t.Fatalf("ReplaceRoleInScope() error: %v", err)
	}

	if replacement.ID != original.ID {
		t.Fatalf("expected existing role id %s to be preserved, got %s", original.ID, replacement.ID)
	}
	if !replacement.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("expected created_at %s to be preserved, got %s", original.CreatedAt, replacement.CreatedAt)
	}

	roles, err := memberRepo.GetRolesInScope(ctx, member.ID, domain.ScopeWorkspace, &workspace.ID)
	if err != nil {
		t.Fatalf("GetRolesInScope() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 workspace role, got %d", len(roles))
	}
	if roles[0].Role != domain.RoleWorkspaceAdmin {
		t.Fatalf("expected workspace_admin, got %s", roles[0].Role)
	}
}

func TestMemberRepo_ReplaceRoleInScope_IsIdempotentWhenSameRoleAlreadyExists(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, memberRepo)
	original := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleSuperadmin,
		ScopeType: domain.ScopeGlobal,
	}
	if err := memberRepo.AddRole(ctx, original); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	replacement := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleSuperadmin,
		ScopeType: domain.ScopeGlobal,
	}
	if err := memberRepo.ReplaceRoleInScope(ctx, replacement); err != nil {
		t.Fatalf("ReplaceRoleInScope() error: %v", err)
	}

	if replacement.ID != original.ID {
		t.Fatalf("expected role id %s, got %s", original.ID, replacement.ID)
	}
	if !replacement.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("expected created_at %s, got %s", original.CreatedAt, replacement.CreatedAt)
	}

	roles, err := memberRepo.GetRoles(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetRoles() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 role after idempotent replace, got %d", len(roles))
	}
}

func TestMemberRepo_ReplaceRoleInScope_DoesNotTouchOtherScopes(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	workspace := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Code:        "ws-" + uuid.New().String()[:8],
		Name:        "Test WS",
		Environment: domain.EnvironmentProd,
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	tenantRole := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleTenantAdmin,
		ScopeType: domain.ScopeTenant,
		TenantID:  &tenant.ID,
	}
	if err := memberRepo.AddRole(ctx, tenantRole); err != nil {
		t.Fatalf("AddRole() tenant error: %v", err)
	}
	workspaceRole := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceViewer,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.AddRole(ctx, workspaceRole); err != nil {
		t.Fatalf("AddRole() workspace error: %v", err)
	}

	replacement := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceEditor,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.ReplaceRoleInScope(ctx, replacement); err != nil {
		t.Fatalf("ReplaceRoleInScope() error: %v", err)
	}

	tenantRoles, err := memberRepo.GetRolesInScope(ctx, member.ID, domain.ScopeTenant, &tenant.ID)
	if err != nil {
		t.Fatalf("GetRolesInScope(tenant) error: %v", err)
	}
	if len(tenantRoles) != 1 || tenantRoles[0].Role != domain.RoleTenantAdmin {
		t.Fatalf("expected tenant_admin role to remain untouched, got %+v", tenantRoles)
	}

	workspaceRoles, err := memberRepo.GetRolesInScope(ctx, member.ID, domain.ScopeWorkspace, &workspace.ID)
	if err != nil {
		t.Fatalf("GetRolesInScope(workspace) error: %v", err)
	}
	if len(workspaceRoles) != 1 || workspaceRoles[0].Role != domain.RoleWorkspaceEditor {
		t.Fatalf("expected workspace_editor replacement, got %+v", workspaceRoles)
	}
}

func TestMemberRepo_RemoveRole(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(pool)

	member := createTestMember(ctx, t, memberRepo)

	role := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal,
	}
	if err := memberRepo.AddRole(ctx, role); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	if err := memberRepo.RemoveRole(ctx, role.ID); err != nil {
		t.Fatalf("RemoveRole() error: %v", err)
	}

	// Verify role is gone
	roles, err := memberRepo.GetRoles(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetRoles() error: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected 0 roles after removal, got %d", len(roles))
	}
}

func TestMemberRepo_RevokeAccessInScope_Global(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	member := createTestMember(ctx, t, memberRepo)

	globalRole := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleSuperadmin,
		ScopeType: domain.ScopeGlobal,
	}
	if err := memberRepo.AddRole(ctx, globalRole); err != nil {
		t.Fatalf("AddRole() global error: %v", err)
	}

	tenantRole := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleTenantAdmin,
		ScopeType: domain.ScopeTenant,
		TenantID:  &tenant.ID,
	}
	if err := memberRepo.AddRole(ctx, tenantRole); err != nil {
		t.Fatalf("AddRole() tenant error: %v", err)
	}

	removed, err := memberRepo.RevokeAccessInScope(ctx, member.ID, domain.ScopeGlobal, nil)
	if err != nil {
		t.Fatalf("RevokeAccessInScope(global) error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 global role removed, got %d", removed)
	}

	roles, err := memberRepo.GetRoles(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetRoles() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 remaining role, got %d", len(roles))
	}
	if roles[0].ScopeType != domain.ScopeTenant {
		t.Fatalf("expected tenant role to remain, got scope %s", roles[0].ScopeType)
	}
}

func TestMemberRepo_RevokeAccessInScope_Tenant(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	workspace := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Code:        "ws-" + uuid.New().String()[:8],
		Name:        "Test WS",
		Environment: domain.EnvironmentProd,
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	tenantRole := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleTenantAdmin,
		ScopeType: domain.ScopeTenant,
		TenantID:  &tenant.ID,
	}
	if err := memberRepo.AddRole(ctx, tenantRole); err != nil {
		t.Fatalf("AddRole() tenant error: %v", err)
	}

	workspaceRole := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceViewer,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.AddRole(ctx, workspaceRole); err != nil {
		t.Fatalf("AddRole() workspace error: %v", err)
	}

	removed, err := memberRepo.RevokeAccessInScope(ctx, member.ID, domain.ScopeTenant, &tenant.ID)
	if err != nil {
		t.Fatalf("RevokeAccessInScope(tenant) error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 tenant role removed, got %d", removed)
	}

	roles, err := memberRepo.GetRoles(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetRoles() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 remaining role, got %d", len(roles))
	}
	if roles[0].ScopeType != domain.ScopeWorkspace {
		t.Fatalf("expected workspace role to remain, got scope %s", roles[0].ScopeType)
	}
}

func TestMemberRepo_RevokeAccessInScope_Workspace(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	workspace := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Code:        "ws-" + uuid.New().String()[:8],
		Name:        "Test WS",
		Environment: domain.EnvironmentProd,
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	workspaceRole := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceEditor,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.AddRole(ctx, workspaceRole); err != nil {
		t.Fatalf("AddRole() workspace error: %v", err)
	}

	tenantRole := &domain.MemberRole{
		ID:        uuid.New(),
		MemberID:  member.ID,
		Role:      domain.RoleTenantAdmin,
		ScopeType: domain.ScopeTenant,
		TenantID:  &tenant.ID,
	}
	if err := memberRepo.AddRole(ctx, tenantRole); err != nil {
		t.Fatalf("AddRole() tenant error: %v", err)
	}

	removed, err := memberRepo.RevokeAccessInScope(ctx, member.ID, domain.ScopeWorkspace, &workspace.ID)
	if err != nil {
		t.Fatalf("RevokeAccessInScope(workspace) error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 workspace role removed, got %d", removed)
	}

	roles, err := memberRepo.GetRoles(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetRoles() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 remaining role, got %d", len(roles))
	}
	if roles[0].ScopeType != domain.ScopeTenant {
		t.Fatalf("expected tenant role to remain, got scope %s", roles[0].ScopeType)
	}
}

func TestMemberRepo_RemoveRole_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewMemberRepo(pool)

	err := repo.RemoveRole(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestMemberRepo_GetRoles(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	member := createTestMember(ctx, t, memberRepo)

	// Add global role
	r1 := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal,
	}
	if err := memberRepo.AddRole(ctx, r1); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	// Add tenant role
	r2 := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeTenant,
		TenantID: &tenant.ID,
	}
	if err := memberRepo.AddRole(ctx, r2); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	roles, err := memberRepo.GetRoles(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetRoles() error: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
}

func TestMemberRepo_GetRolesInScope_Global(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	member := createTestMember(ctx, t, memberRepo)

	// Add global and tenant roles
	r1 := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal,
	}
	r2 := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeTenant,
		TenantID: &tenant.ID,
	}
	if err := memberRepo.AddRole(ctx, r1); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}
	if err := memberRepo.AddRole(ctx, r2); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	// Query global scope only
	roles, err := memberRepo.GetRolesInScope(ctx, member.ID, domain.ScopeGlobal, nil)
	if err != nil {
		t.Fatalf("GetRolesInScope() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 global role, got %d", len(roles))
	}
	if roles[0].Role != domain.RoleSuperadmin {
		t.Errorf("expected superadmin, got %s", roles[0].Role)
	}
}

func TestMemberRepo_GetRolesInScope_Tenant(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	member := createTestMember(ctx, t, memberRepo)

	role := &domain.MemberRole{
		ID: uuid.New(), MemberID: member.ID,
		Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeTenant,
		TenantID: &tenant.ID,
	}
	if err := memberRepo.AddRole(ctx, role); err != nil {
		t.Fatalf("AddRole() error: %v", err)
	}

	roles, err := memberRepo.GetRolesInScope(ctx, member.ID, domain.ScopeTenant, &tenant.ID)
	if err != nil {
		t.Fatalf("GetRolesInScope() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 tenant role, got %d", len(roles))
	}
}

func TestMemberRolesSingleScopeMigration_NormalizesToHighestRolePerScope(t *testing.T) {
	ctx := context.Background()
	sharedDBMu.Lock()
	t.Cleanup(func() {
		sharedDBMu.Unlock()
	})
	connStr := sharedConnStr(ctx, t)

	if err := pgadapter.RunMigrationsDown(connStr, migrationsPath()); err != nil {
		t.Fatalf("resetting migrations down: %v", err)
	}

	m, err := migrate.New("file://"+migrationsPath(), pgadapterTestPGXURL(connStr))
	if err != nil {
		t.Fatalf("creating migration instance: %v", err)
	}
	t.Cleanup(func() {
		_, _ = m.Close()
	})

	if err := m.Migrate(44); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrating to version 44: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	memberRepo := pgadapter.NewMemberRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	tenant := createTestTenant(ctx, t, tenantRepo)
	workspace := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Code:        "ws-" + uuid.New().String()[:8],
		Name:        "Test WS",
		Environment: domain.EnvironmentProd,
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	viewer := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceViewer,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.AddRole(ctx, viewer); err != nil {
		t.Fatalf("adding viewer role: %v", err)
	}
	editor := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceEditor,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.AddRole(ctx, editor); err != nil {
		t.Fatalf("adding editor role: %v", err)
	}
	admin := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceAdmin,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	if err := memberRepo.AddRole(ctx, admin); err != nil {
		t.Fatalf("adding admin role: %v", err)
	}

	if err := m.Steps(1); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("applying migration 45: %v", err)
	}

	roles, err := memberRepo.GetRolesInScope(ctx, member.ID, domain.ScopeWorkspace, &workspace.ID)
	if err != nil {
		t.Fatalf("GetRolesInScope() error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 normalized role, got %d", len(roles))
	}
	if roles[0].Role != domain.RoleWorkspaceAdmin {
		t.Fatalf("expected highest role workspace_admin to remain, got %s", roles[0].Role)
	}

	conflict := &domain.MemberRole{
		ID:          uuid.New(),
		MemberID:    member.ID,
		Role:        domain.RoleWorkspaceViewer,
		ScopeType:   domain.ScopeWorkspace,
		TenantID:    &tenant.ID,
		WorkspaceID: &workspace.ID,
	}
	err = memberRepo.AddRole(ctx, conflict)
	if err == nil {
		t.Fatal("expected conflict when inserting a second local role in the same workspace scope")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Fatalf("expected 409 conflict, got %v", err)
	}
}

func pgadapterTestPGXURL(dbURL string) string {
	return fmt.Sprintf("pgx5://%s", strings.TrimPrefix(strings.TrimPrefix(dbURL, "postgres://"), "postgresql://"))
}

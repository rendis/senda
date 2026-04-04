//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
)

// createTestTenant is a helper that creates a tenant for workspace tests.
func createTestTenant(ctx context.Context, t *testing.T, repo *pgadapter.TenantRepo) *domain.Tenant {
	t.Helper()
	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "t-" + uuid.New().String()[:8],
		Name: "Test Tenant",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("creating test tenant: %v", err)
	}
	return tenant
}

func TestWorkspaceRepo_Create(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	ws := &domain.Workspace{
		ID:                  uuid.New(),
		TenantID:            tenant.ID,
		Code:                "main",
		Name:                "Main Workspace",
		IsSystem:            false,
		OpenTrackingEnabled: true,
	}

	if err := repo.Create(ctx, ws); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if ws.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if ws.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if !ws.IsActive {
		t.Error("expected IsActive to default to true after create")
	}
}

func TestWorkspaceRepo_Create_DuplicateTenantCode(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	ws1 := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "dup-ws",
		Name:     "First",
	}
	if err := repo.Create(ctx, ws1); err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	ws2 := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "dup-ws",
		Name:     "Second",
	}
	err := repo.Create(ctx, ws2)
	if err == nil {
		t.Fatal("expected conflict error for duplicate (tenant_id, code)")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestWorkspaceRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	locale := "en"

	ws := &domain.Workspace{
		ID:                  uuid.New(),
		TenantID:            tenant.ID,
		Code:                "get-id",
		Name:                "GetByID WS",
		OpenTrackingEnabled: true,
		DefaultLocale:       &locale,
	}
	if err := repo.Create(ctx, ws); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByID(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Code != "get-id" || got.Name != "GetByID WS" {
		t.Errorf("unexpected workspace: %+v", got)
	}
	if !got.OpenTrackingEnabled {
		t.Error("expected OpenTrackingEnabled to be true")
	}
	if got.DefaultLocale == nil || *got.DefaultLocale != "en" {
		t.Error("expected DefaultLocale to be 'en'")
	}
	if !got.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestWorkspaceRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewWorkspaceRepo(pool)

	_, err := repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWorkspaceRepo_GetByTenantAndCode(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	ws := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "tc-lookup",
		Name:     "TC Lookup",
	}
	if err := repo.Create(ctx, ws); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByTenantAndCode(ctx, tenant.ID, "tc-lookup")
	if err != nil {
		t.Fatalf("GetByTenantAndCode() error: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("want ID %s, got %s", ws.ID, got.ID)
	}
}

func TestWorkspaceRepo_GetByTenantAndCode_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewWorkspaceRepo(pool)

	_, err := repo.GetByTenantAndCode(ctx, uuid.New(), "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWorkspaceRepo_GetSystemWorkspace(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	sysWS := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "_system",
		Name:     "System",
		IsSystem: true,
	}
	if err := repo.Create(ctx, sysWS); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetSystemWorkspace(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetSystemWorkspace() error: %v", err)
	}
	if got.ID != sysWS.ID || !got.IsSystem {
		t.Errorf("unexpected system workspace: %+v", got)
	}
}

func TestWorkspaceRepo_GetSystemWorkspace_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewWorkspaceRepo(pool)

	_, err := repo.GetSystemWorkspace(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWorkspaceRepo_Update(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	ws := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "upd-ws",
		Name:     "Original",
	}
	if err := repo.Create(ctx, ws); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	originalUpdatedAt := ws.UpdatedAt
	locale := "es"
	ws.Name = "Updated"
	ws.IsActive = false
	ws.OpenTrackingEnabled = true
	ws.DefaultLocale = &locale

	if err := repo.Update(ctx, ws); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if !ws.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to advance")
	}

	got, err := repo.GetByID(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("want name Updated, got %q", got.Name)
	}
	if !got.OpenTrackingEnabled {
		t.Error("expected OpenTrackingEnabled to be true")
	}
	if got.DefaultLocale == nil || *got.DefaultLocale != "es" {
		t.Errorf("expected DefaultLocale 'es', got %v", got.DefaultLocale)
	}
	if got.IsActive {
		t.Error("expected IsActive to be false")
	}
}

func TestWorkspaceRepo_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewWorkspaceRepo(pool)

	ws := &domain.Workspace{
		ID:   uuid.New(),
		Name: "Ghost",
	}
	err := repo.Update(ctx, ws)
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWorkspaceRepo_SoftDelete(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	ws := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "del-ws",
		Name:     "To Delete",
	}
	if err := repo.Create(ctx, ws); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.SoftDelete(ctx, ws.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	// Not visible via GetByID
	_, err := repo.GetByID(ctx, ws.ID)
	if err == nil {
		t.Fatal("expected not found after soft delete")
	}

	// Still in DB
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM workspaces WHERE id = $1", ws.ID).Scan(&count); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 1 {
		t.Errorf("expected row to still exist, count=%d", count)
	}
}

func TestWorkspaceRepo_SoftDelete_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewWorkspaceRepo(pool)

	err := repo.SoftDelete(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWorkspaceRepo_ListByTenant(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	// Create 5 workspaces
	for i := range 5 {
		ws := &domain.Workspace{
			ID:       uuid.New(),
			TenantID: tenant.ID,
			Code:     "ws-" + uuid.New().String()[:8],
			Name:     "Workspace",
		}
		if err := repo.Create(ctx, ws); err != nil {
			t.Fatalf("Create(%d) error: %v", i, err)
		}
	}

	// First page
	page1, cursor1, err := repo.ListByTenant(ctx, tenant.ID, port.ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListByTenant(page1) error: %v", err)
	}
	if len(page1) != 3 {
		t.Fatalf("expected 3 items on page 1, got %d", len(page1))
	}
	if cursor1 == "" {
		t.Fatal("expected non-empty cursor for page 1")
	}

	// Second page
	page2, cursor2, err := repo.ListByTenant(ctx, tenant.ID, port.ListOptions{Limit: 3, Cursor: cursor1})
	if err != nil {
		t.Fatalf("ListByTenant(page2) error: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(page2))
	}
	if cursor2 != "" {
		t.Error("expected empty cursor for last page")
	}

	// Verify all belong to the same tenant
	for _, ws := range append(page1, page2...) {
		if ws.TenantID != tenant.ID {
			t.Errorf("workspace %s has wrong tenant_id %s", ws.ID, ws.TenantID)
		}
	}
}

func TestWorkspaceRepo_ListByTenant_SoftDeletedExcluded(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	ws := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "list-del-ws",
		Name:     "Deleted",
	}
	if err := repo.Create(ctx, ws); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := repo.SoftDelete(ctx, ws.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	results, _, err := repo.ListByTenant(ctx, tenant.ID, port.ListOptions{})
	if err != nil {
		t.Fatalf("ListByTenant() error: %v", err)
	}
	for _, r := range results {
		if r.ID == ws.ID {
			t.Error("soft-deleted workspace should not appear in ListByTenant")
		}
	}
}

func TestWorkspaceRepo_ListByTenant_IsolatesTenants(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	repo := pgadapter.NewWorkspaceRepo(pool)

	tenant1 := createTestTenant(ctx, t, tenantRepo)
	tenant2 := createTestTenant(ctx, t, tenantRepo)

	// Create workspace for tenant1
	ws1 := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant1.ID,
		Code:     "iso-ws",
		Name:     "Tenant1 WS",
	}
	if err := repo.Create(ctx, ws1); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// List for tenant2 should be empty
	results, _, err := repo.ListByTenant(ctx, tenant2.ID, port.ListOptions{})
	if err != nil {
		t.Fatalf("ListByTenant() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 workspaces for tenant2, got %d", len(results))
	}
}

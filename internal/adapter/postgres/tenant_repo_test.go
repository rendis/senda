//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	pgadapter "github.com/senda-app/senda/internal/adapter/postgres"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/apperr"
)

func TestTenantRepo_Create(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "acme",
		Name: "Acme Corp",
	}

	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if tenant.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if tenant.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestTenantRepo_Create_DuplicateCode(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "dup-code",
		Name: "First",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	dup := &domain.Tenant{
		ID:   uuid.New(),
		Code: "dup-code",
		Name: "Second",
	}
	err := repo.Create(ctx, dup)
	if err == nil {
		t.Fatal("expected conflict error for duplicate code")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestTenantRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "byid",
		Name: "By ID",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Code != "byid" || got.Name != "By ID" {
		t.Errorf("unexpected tenant: %+v", got)
	}
}

func TestTenantRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	_, err := repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404 Not Found, got: %v", err)
	}
}

func TestTenantRepo_GetByCode(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "bycode",
		Name: "By Code",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByCode(ctx, "bycode")
	if err != nil {
		t.Fatalf("GetByCode() error: %v", err)
	}
	if got.ID != tenant.ID {
		t.Errorf("want ID %s, got %s", tenant.ID, got.ID)
	}
}

func TestTenantRepo_GetByCode_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	_, err := repo.GetByCode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404 Not Found, got: %v", err)
	}
}

func TestTenantRepo_Update(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "upd",
		Name: "Original",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	originalUpdatedAt := tenant.UpdatedAt
	tenant.Name = "Updated"

	if err := repo.Update(ctx, tenant); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if !tenant.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to advance after update")
	}

	got, err := repo.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("want name Updated, got %q", got.Name)
	}
}

func TestTenantRepo_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Name: "Ghost",
	}
	err := repo.Update(ctx, tenant)
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestTenantRepo_SoftDelete(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "softdel",
		Name: "To Delete",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.SoftDelete(ctx, tenant.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	// Should not be found via GetByID
	_, err := repo.GetByID(ctx, tenant.ID)
	if err == nil {
		t.Fatal("expected not found after soft delete")
	}

	// But row still exists in DB
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants WHERE id = $1", tenant.ID).Scan(&count); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 1 {
		t.Errorf("expected row to still exist in DB, count=%d", count)
	}
}

func TestTenantRepo_SoftDelete_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	err := repo.SoftDelete(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestTenantRepo_Purge(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "purge",
		Name: "To Purge",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.Purge(ctx, tenant.ID); err != nil {
		t.Fatalf("Purge() error: %v", err)
	}

	// Row should be completely gone
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants WHERE id = $1", tenant.ID).Scan(&count); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 0 {
		t.Errorf("expected row to be deleted from DB, count=%d", count)
	}
}

func TestTenantRepo_Purge_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	err := repo.Purge(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestTenantRepo_List(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	// Create 5 tenants
	ids := make([]uuid.UUID, 5)
	for i := range 5 {
		id := uuid.New()
		ids[i] = id
		tenant := &domain.Tenant{
			ID:   id,
			Code: "list-" + id.String()[:8],
			Name: "Tenant " + id.String()[:8],
		}
		if err := repo.Create(ctx, tenant); err != nil {
			t.Fatalf("Create(%d) error: %v", i, err)
		}
	}

	// First page: 3 items
	page1, cursor1, err := repo.List(ctx, port.ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("List(page1) error: %v", err)
	}
	if len(page1) != 3 {
		t.Fatalf("expected 3 items on page 1, got %d", len(page1))
	}
	if cursor1 == "" {
		t.Fatal("expected non-empty cursor for page 1")
	}

	// Second page: remaining items
	page2, cursor2, err := repo.List(ctx, port.ListOptions{Limit: 3, Cursor: cursor1})
	if err != nil {
		t.Fatalf("List(page2) error: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(page2))
	}
	if cursor2 != "" {
		t.Error("expected empty cursor for last page")
	}

	// Verify no overlap
	seen := make(map[uuid.UUID]bool)
	for _, tn := range page1 {
		seen[tn.ID] = true
	}
	for _, tn := range page2 {
		if seen[tn.ID] {
			t.Errorf("tenant %s appears in both pages", tn.ID)
		}
	}
}

func TestTenantRepo_List_SoftDeletedExcluded(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTenantRepo(pool)

	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "list-del",
		Name: "Deleted Tenant",
	}
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := repo.SoftDelete(ctx, tenant.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	results, _, err := repo.List(ctx, port.ListOptions{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	for _, r := range results {
		if r.ID == tenant.ID {
			t.Error("soft-deleted tenant should not appear in List")
		}
	}
}

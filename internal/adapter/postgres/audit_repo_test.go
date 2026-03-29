//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

func newTestAuditLog(tenantID *uuid.UUID) *domain.AuditLog {
	return &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    uuid.New(),
		ActorEmail: "actor@test.com",
		Action:     domain.AuditCreate,
		EntityType: "tenant",
		EntityID:   uuid.New(),
		ScopeType:  domain.ScopeTenant,
		TenantID:   tenantID,
	}
}

func TestAuditRepo_Append(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAuditRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	entry := newTestAuditLog(&tenant.ID)
	entry.Changes = map[string]any{"name": "new_name"}
	entry.Metadata = map[string]any{"ip": "127.0.0.1"}

	if err := repo.Append(ctx, entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestAuditRepo_Query_All(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAuditRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	// Create 3 entries
	for range 3 {
		entry := newTestAuditLog(&tenant.ID)
		if err := repo.Append(ctx, entry); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
		time.Sleep(time.Millisecond)
	}

	result, err := repo.Query(ctx, port.AuditFilter{TenantID: &tenant.ID}, port.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	if result.HasMore {
		t.Error("expected HasMore to be false")
	}
}

func TestAuditRepo_Query_Pagination(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAuditRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	// Create 5 entries
	for range 5 {
		entry := newTestAuditLog(&tenant.ID)
		if err := repo.Append(ctx, entry); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
		time.Sleep(time.Millisecond)
	}

	// Page 1
	result1, err := repo.Query(ctx, port.AuditFilter{TenantID: &tenant.ID}, port.ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Query(page1) error: %v", err)
	}
	if len(result1.Items) != 3 {
		t.Fatalf("expected 3 on page 1, got %d", len(result1.Items))
	}
	if !result1.HasMore {
		t.Error("expected HasMore on page 1")
	}

	// Page 2
	result2, err := repo.Query(ctx, port.AuditFilter{TenantID: &tenant.ID}, port.ListOptions{Limit: 3, Cursor: result1.NextCursor})
	if err != nil {
		t.Fatalf("Query(page2) error: %v", err)
	}
	if len(result2.Items) != 2 {
		t.Fatalf("expected 2 on page 2, got %d", len(result2.Items))
	}
	if result2.HasMore {
		t.Error("expected no more on page 2")
	}

	// Verify no overlap
	seen := make(map[uuid.UUID]bool)
	for _, item := range result1.Items {
		seen[item.ID] = true
	}
	for _, item := range result2.Items {
		if seen[item.ID] {
			t.Errorf("audit log %s appears on both pages", item.ID)
		}
	}
}

func TestAuditRepo_Query_FilterByAction(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAuditRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	// Create entries with different actions
	for _, action := range []domain.AuditAction{domain.AuditCreate, domain.AuditUpdate, domain.AuditCreate} {
		entry := newTestAuditLog(&tenant.ID)
		entry.Action = action
		if err := repo.Append(ctx, entry); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	action := string(domain.AuditCreate)
	result, err := repo.Query(ctx, port.AuditFilter{
		TenantID: &tenant.ID,
		Action:   &action,
	}, port.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Query(action=create) error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 create entries, got %d", len(result.Items))
	}
}

func TestAuditRepo_Query_FilterByActorID(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAuditRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	actorID := uuid.New()

	entry := newTestAuditLog(&tenant.ID)
	entry.ActorID = actorID
	if err := repo.Append(ctx, entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Another entry with different actor
	other := newTestAuditLog(&tenant.ID)
	if err := repo.Append(ctx, other); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	result, err := repo.Query(ctx, port.AuditFilter{
		TenantID: &tenant.ID,
		ActorID:  &actorID,
	}, port.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Query(actorID) error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 entry for actor, got %d", len(result.Items))
	}
}

func TestAuditRepo_Query_FilterByEntityType(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAuditRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	entry1 := newTestAuditLog(&tenant.ID)
	entry1.EntityType = "workspace"
	if err := repo.Append(ctx, entry1); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	entry2 := newTestAuditLog(&tenant.ID)
	entry2.EntityType = "template"
	if err := repo.Append(ctx, entry2); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	entityType := "workspace"
	result, err := repo.Query(ctx, port.AuditFilter{
		TenantID:   &tenant.ID,
		EntityType: &entityType,
	}, port.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Query(entityType) error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 workspace entry, got %d", len(result.Items))
	}
}

func TestAuditRepo_Query_FilterByTimeRange(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAuditRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)

	entry := newTestAuditLog(&tenant.ID)
	if err := repo.Append(ctx, entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Query with future "since" — should return 0
	future := time.Now().Add(1 * time.Hour)
	result, err := repo.Query(ctx, port.AuditFilter{
		TenantID: &tenant.ID,
		Since:    &future,
	}, port.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Query(future since) error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items with future since, got %d", len(result.Items))
	}
}

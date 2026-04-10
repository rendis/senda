//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

func TestWorkspaceRepo_ListByTenant_DoesNotRequireNonPersistedFields(t *testing.T) {
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
		Environment:         domain.EnvironmentProd,
		OpenTrackingEnabled: true,
	}
	if err := repo.Create(ctx, ws); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	workspaces, _, err := repo.ListByTenant(ctx, tenant.ID, domain.EnvironmentProd, port.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListByTenant() error: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(workspaces))
	}
	if !workspaces[0].WorkspacePoliciesInitialized {
		t.Fatal("expected workspace policies to be marked as initialized after scan")
	}
}

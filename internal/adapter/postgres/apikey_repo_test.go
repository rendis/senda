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

func TestAPIKeyRepo_Create(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)
	repo := pgadapter.NewAPIKeyRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	key := &domain.APIKey{
		ID:          uuid.New(),
		WorkspaceID: ws.ID,
		Name:        "Test Key",
		KeyHash:     "sha256_" + uuid.New().String(),
		KeyPrefix:   "senda_live",
		KeyHint:     "abcd1234",
		CreatedBy:   member.ID,
	}

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if key.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestAPIKeyRepo_Create_DuplicateHash(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)
	repo := pgadapter.NewAPIKeyRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	hash := "dup_hash_" + uuid.New().String()

	k1 := &domain.APIKey{
		ID: uuid.New(), WorkspaceID: ws.ID, Name: "Key 1",
		KeyHash: hash, KeyPrefix: "senda_live", KeyHint: "hint1234",
		CreatedBy: member.ID,
	}
	if err := repo.Create(ctx, k1); err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	k2 := &domain.APIKey{
		ID: uuid.New(), WorkspaceID: ws.ID, Name: "Key 2",
		KeyHash: hash, KeyPrefix: "senda_live", KeyHint: "hint5678",
		CreatedBy: member.ID,
	}
	err := repo.Create(ctx, k2)
	if err == nil {
		t.Fatal("expected conflict error for duplicate hash")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestAPIKeyRepo_GetByHash(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)
	repo := pgadapter.NewAPIKeyRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	hash := "hash_" + uuid.New().String()
	key := &domain.APIKey{
		ID: uuid.New(), WorkspaceID: ws.ID, Name: "Test Key",
		KeyHash: hash, KeyPrefix: "senda_live", KeyHint: "abcd1234",
		CreatedBy: member.ID,
	}
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByHash() error: %v", err)
	}
	if got.ID != key.ID {
		t.Errorf("want ID %s, got %s", key.ID, got.ID)
	}
	if got.KeyHash != hash {
		t.Errorf("want KeyHash %s, got %s", hash, got.KeyHash)
	}
}

func TestAPIKeyRepo_GetByHash_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAPIKeyRepo(pool)

	_, err := repo.GetByHash(ctx, "nonexistent_hash")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestAPIKeyRepo_GetByHash_RevokedExcluded(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)
	repo := pgadapter.NewAPIKeyRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	hash := "revoke_hash_" + uuid.New().String()
	key := &domain.APIKey{
		ID: uuid.New(), WorkspaceID: ws.ID, Name: "Test Key",
		KeyHash: hash, KeyPrefix: "senda_live", KeyHint: "abcd1234",
		CreatedBy: member.ID,
	}
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("Revoke() error: %v", err)
	}

	_, err := repo.GetByHash(ctx, hash)
	if err == nil {
		t.Fatal("expected not found after revoke")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestAPIKeyRepo_Revoke(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)
	repo := pgadapter.NewAPIKeyRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	key := &domain.APIKey{
		ID: uuid.New(), WorkspaceID: ws.ID, Name: "Test Key",
		KeyHash: "rk_" + uuid.New().String(), KeyPrefix: "senda_live", KeyHint: "abcd1234",
		CreatedBy: member.ID,
	}
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("Revoke() error: %v", err)
	}

	// Revoking again should fail
	err := repo.Revoke(ctx, key.ID)
	if err == nil {
		t.Fatal("expected not found on double revoke")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestAPIKeyRepo_Revoke_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAPIKeyRepo(pool)

	err := repo.Revoke(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestAPIKeyRepo_TouchLastUsed(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)
	repo := pgadapter.NewAPIKeyRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	hash := "touch_" + uuid.New().String()
	key := &domain.APIKey{
		ID: uuid.New(), WorkspaceID: ws.ID, Name: "Test Key",
		KeyHash: hash, KeyPrefix: "senda_live", KeyHint: "abcd1234",
		CreatedBy: member.ID,
	}
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.TouchLastUsed(ctx, key.ID); err != nil {
		t.Fatalf("TouchLastUsed() error: %v", err)
	}

	got, err := repo.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByHash() error: %v", err)
	}
	if got.LastUsedAt == nil {
		t.Error("expected LastUsedAt to be set after TouchLastUsed")
	}
}

func TestAPIKeyRepo_ListByWorkspace(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	memberRepo := pgadapter.NewMemberRepo(pool)
	repo := pgadapter.NewAPIKeyRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	member := createTestMember(ctx, t, memberRepo)

	// Create 5 keys
	for i := range 5 {
		key := &domain.APIKey{
			ID: uuid.New(), WorkspaceID: ws.ID, Name: "Key",
			KeyHash: "list_" + uuid.New().String(), KeyPrefix: "senda_live",
			KeyHint: "hint1234", CreatedBy: member.ID,
		}
		if err := repo.Create(ctx, key); err != nil {
			t.Fatalf("Create(%d) error: %v", i, err)
		}
	}

	// First page
	result, err := repo.ListByWorkspace(ctx, ws.ID, port.ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListByWorkspace() error: %v", err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	if !result.HasMore {
		t.Error("expected HasMore to be true")
	}

	// Verify key_hash is excluded
	for _, k := range result.Items {
		if k.KeyHash != "" {
			t.Error("expected KeyHash to be empty in list results")
		}
	}

	// Second page
	result2, err := repo.ListByWorkspace(ctx, ws.ID, port.ListOptions{Limit: 3, Cursor: result.NextCursor})
	if err != nil {
		t.Fatalf("ListByWorkspace(page2) error: %v", err)
	}
	if len(result2.Items) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(result2.Items))
	}
	if result2.HasMore {
		t.Error("expected HasMore to be false on last page")
	}
}

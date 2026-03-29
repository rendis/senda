//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

type suppressionTestDeps struct {
	pool *pgxpool.Pool
	repo *pgadapter.SuppressionRepo
	wsID uuid.UUID
}

func setupSuppressionTestDeps(ctx context.Context, t *testing.T) suppressionTestDeps {
	t.Helper()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws-" + uuid.New().String()[:8], Name: "Test WS",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}

	return suppressionTestDeps{
		pool: pool,
		repo: pgadapter.NewSuppressionRepo(pool),
		wsID: ws.ID,
	}
}

func TestSuppressionRepo_AddGlobal(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	entry := &domain.SuppressionGlobal{
		ID:     uuid.New(),
		Email:  "bounced@test.com",
		Reason: domain.SuppressionHardBounce,
	}

	if err := deps.repo.AddGlobal(ctx, entry); err != nil {
		t.Fatalf("AddGlobal() error: %v", err)
	}

	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestSuppressionRepo_AddGlobal_Upsert(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "upsert@test.com"

	e1 := &domain.SuppressionGlobal{
		ID: uuid.New(), Email: email,
		Reason: domain.SuppressionHardBounce,
	}
	if err := deps.repo.AddGlobal(ctx, e1); err != nil {
		t.Fatalf("first AddGlobal() error: %v", err)
	}

	// Upsert with different reason
	e2 := &domain.SuppressionGlobal{
		ID: uuid.New(), Email: email,
		Reason: domain.SuppressionComplaint,
	}
	if err := deps.repo.AddGlobal(ctx, e2); err != nil {
		t.Fatalf("upsert AddGlobal() error: %v", err)
	}

	// Should still be suppressed
	suppressed, err := deps.repo.IsGloballySuppressed(ctx, email)
	if err != nil {
		t.Fatalf("IsGloballySuppressed() error: %v", err)
	}
	if !suppressed {
		t.Error("expected email to be suppressed after upsert")
	}
}

func TestSuppressionRepo_IsGloballySuppressed(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "check@test.com"

	// Not suppressed initially
	suppressed, err := deps.repo.IsGloballySuppressed(ctx, email)
	if err != nil {
		t.Fatalf("IsGloballySuppressed() error: %v", err)
	}
	if suppressed {
		t.Error("expected not suppressed initially")
	}

	// Add suppression
	entry := &domain.SuppressionGlobal{
		ID: uuid.New(), Email: email,
		Reason: domain.SuppressionManual,
	}
	if err := deps.repo.AddGlobal(ctx, entry); err != nil {
		t.Fatalf("AddGlobal() error: %v", err)
	}

	// Now suppressed
	suppressed, err = deps.repo.IsGloballySuppressed(ctx, email)
	if err != nil {
		t.Fatalf("IsGloballySuppressed() error: %v", err)
	}
	if !suppressed {
		t.Error("expected suppressed after AddGlobal")
	}
}

func TestSuppressionRepo_RemoveGlobal(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(deps.pool)
	member := createTestMember(ctx, t, memberRepo)

	email := "remove@test.com"

	entry := &domain.SuppressionGlobal{
		ID: uuid.New(), Email: email,
		Reason: domain.SuppressionHardBounce,
	}
	if err := deps.repo.AddGlobal(ctx, entry); err != nil {
		t.Fatalf("AddGlobal() error: %v", err)
	}

	if err := deps.repo.RemoveGlobal(ctx, email, member.ID, "false positive"); err != nil {
		t.Fatalf("RemoveGlobal() error: %v", err)
	}

	// Should no longer be suppressed
	suppressed, err := deps.repo.IsGloballySuppressed(ctx, email)
	if err != nil {
		t.Fatalf("IsGloballySuppressed() error: %v", err)
	}
	if suppressed {
		t.Error("expected not suppressed after removal")
	}
}

func TestSuppressionRepo_RemoveGlobal_NotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	err := deps.repo.RemoveGlobal(ctx, "nonexistent@test.com", uuid.New(), "reason")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestSuppressionRepo_AddWorkspace(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	entry := &domain.SuppressionWorkspace{
		ID: uuid.New(), WorkspaceID: deps.wsID,
		Email: "ws-bounced@test.com", Reason: domain.SuppressionHardBounce,
	}

	if err := deps.repo.AddWorkspace(ctx, entry); err != nil {
		t.Fatalf("AddWorkspace() error: %v", err)
	}

	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestSuppressionRepo_IsWorkspaceSuppressed(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "ws-check@test.com"

	suppressed, err := deps.repo.IsWorkspaceSuppressed(ctx, deps.wsID, email)
	if err != nil {
		t.Fatalf("IsWorkspaceSuppressed() error: %v", err)
	}
	if suppressed {
		t.Error("expected not suppressed initially")
	}

	entry := &domain.SuppressionWorkspace{
		ID: uuid.New(), WorkspaceID: deps.wsID,
		Email: email, Reason: domain.SuppressionComplaint,
	}
	if err := deps.repo.AddWorkspace(ctx, entry); err != nil {
		t.Fatalf("AddWorkspace() error: %v", err)
	}

	suppressed, err = deps.repo.IsWorkspaceSuppressed(ctx, deps.wsID, email)
	if err != nil {
		t.Fatalf("IsWorkspaceSuppressed() error: %v", err)
	}
	if !suppressed {
		t.Error("expected suppressed after AddWorkspace")
	}
}

func TestSuppressionRepo_IsSuppressed_GlobalMatch(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "combined-global@test.com"

	entry := &domain.SuppressionGlobal{
		ID: uuid.New(), Email: email,
		Reason: domain.SuppressionHardBounce,
	}
	if err := deps.repo.AddGlobal(ctx, entry); err != nil {
		t.Fatalf("AddGlobal() error: %v", err)
	}

	suppressed, reason, err := deps.repo.IsSuppressed(ctx, deps.wsID, email)
	if err != nil {
		t.Fatalf("IsSuppressed() error: %v", err)
	}
	if !suppressed {
		t.Error("expected suppressed")
	}
	if reason != string(domain.SuppressionHardBounce) {
		t.Errorf("expected reason hard_bounce, got %s", reason)
	}
}

func TestSuppressionRepo_IsSuppressed_WorkspaceMatch(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "combined-ws@test.com"

	entry := &domain.SuppressionWorkspace{
		ID: uuid.New(), WorkspaceID: deps.wsID,
		Email: email, Reason: domain.SuppressionComplaint,
	}
	if err := deps.repo.AddWorkspace(ctx, entry); err != nil {
		t.Fatalf("AddWorkspace() error: %v", err)
	}

	suppressed, reason, err := deps.repo.IsSuppressed(ctx, deps.wsID, email)
	if err != nil {
		t.Fatalf("IsSuppressed() error: %v", err)
	}
	if !suppressed {
		t.Error("expected suppressed")
	}
	if reason != string(domain.SuppressionComplaint) {
		t.Errorf("expected reason complaint, got %s", reason)
	}
}

func TestSuppressionRepo_IsSuppressed_NotSuppressed(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	suppressed, reason, err := deps.repo.IsSuppressed(ctx, deps.wsID, "clean@test.com")
	if err != nil {
		t.Fatalf("IsSuppressed() error: %v", err)
	}
	if suppressed {
		t.Error("expected not suppressed")
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %s", reason)
	}
}

func TestSuppressionRepo_AddGlobal_ReactivatesRemoved(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(deps.pool)
	member := createTestMember(ctx, t, memberRepo)

	email := "reactivate@test.com"

	// Add, then remove
	entry := &domain.SuppressionGlobal{
		ID: uuid.New(), Email: email,
		Reason: domain.SuppressionHardBounce,
	}
	if err := deps.repo.AddGlobal(ctx, entry); err != nil {
		t.Fatalf("AddGlobal() error: %v", err)
	}
	if err := deps.repo.RemoveGlobal(ctx, email, member.ID, "false positive"); err != nil {
		t.Fatalf("RemoveGlobal() error: %v", err)
	}

	// Re-add should reactivate
	entry2 := &domain.SuppressionGlobal{
		ID: uuid.New(), Email: email,
		Reason: domain.SuppressionComplaint,
	}
	if err := deps.repo.AddGlobal(ctx, entry2); err != nil {
		t.Fatalf("re-AddGlobal() error: %v", err)
	}

	suppressed, err := deps.repo.IsGloballySuppressed(ctx, email)
	if err != nil {
		t.Fatalf("IsGloballySuppressed() error: %v", err)
	}
	if !suppressed {
		t.Error("expected suppressed after reactivation")
	}
}

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

func TestSuppressionRepo_GetSuppressionStatuses(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	if err := deps.repo.AddGlobal(ctx, &domain.SuppressionGlobal{
		ID:     uuid.New(),
		Email:  "global@test.com",
		Reason: domain.SuppressionHardBounce,
	}); err != nil {
		t.Fatalf("AddGlobal() error: %v", err)
	}

	if err := deps.repo.AddWorkspace(ctx, &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       "workspace@test.com",
		Reason:      domain.SuppressionComplaint,
	}); err != nil {
		t.Fatalf("AddWorkspace() error: %v", err)
	}

	statuses, err := deps.repo.GetSuppressionStatuses(ctx, deps.wsID, []string{
		"clean@test.com",
		"global@test.com",
		"workspace@test.com",
		"workspace@test.com",
	})
	if err != nil {
		t.Fatalf("GetSuppressionStatuses() error: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("expected 3 unique suppression results, got %d", len(statuses))
	}
	if got := statuses["clean@test.com"]; got.Suppressed || got.Reason != "" {
		t.Fatalf("expected clean@test.com to be clean, got %+v", got)
	}
	if got := statuses["global@test.com"]; !got.Suppressed || got.Reason != string(domain.SuppressionHardBounce) {
		t.Fatalf("expected global suppression, got %+v", got)
	}
	if got := statuses["workspace@test.com"]; !got.Suppressed || got.Reason != string(domain.SuppressionComplaint) {
		t.Fatalf("expected workspace suppression, got %+v", got)
	}
}

func TestSuppressionRepo_GetActiveWorkspaceSuppression_FoundAndNotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "active-ws@test.com"

	// Not found before any suppression is added
	_, err := deps.repo.GetActiveWorkspaceSuppression(ctx, deps.wsID, email)
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("expected 404, got: %v", err)
	}

	// Add workspace suppression
	entry := &domain.SuppressionWorkspace{
		ID: uuid.New(), WorkspaceID: deps.wsID,
		Email: email, Reason: domain.SuppressionUnsubscribe,
	}
	if err := deps.repo.AddWorkspace(ctx, entry); err != nil {
		t.Fatalf("AddWorkspace() error: %v", err)
	}

	// Now found
	sup, err := deps.repo.GetActiveWorkspaceSuppression(ctx, deps.wsID, email)
	if err != nil {
		t.Fatalf("GetActiveWorkspaceSuppression() error: %v", err)
	}
	if sup.Email != email {
		t.Errorf("Email = %q, want %q", sup.Email, email)
	}
	if sup.Reason != domain.SuppressionUnsubscribe {
		t.Errorf("Reason = %q, want unsubscribe", sup.Reason)
	}
	if sup.RemovedAt != nil {
		t.Error("RemovedAt must be nil for active row")
	}

	// Different email returns NotFound
	_, err = deps.repo.GetActiveWorkspaceSuppression(ctx, deps.wsID, "other@test.com")
	if err == nil {
		t.Fatal("expected NotFound for different email, got nil")
	}
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("expected 404 for other email, got: %v", err)
	}
}

func TestSuppressionRepo_RemoveWorkspaceSuppression_SetsRemovedAt(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "to-remove-ws@test.com"

	entry := &domain.SuppressionWorkspace{
		ID: uuid.New(), WorkspaceID: deps.wsID,
		Email: email, Reason: domain.SuppressionUnsubscribe,
	}
	if err := deps.repo.AddWorkspace(ctx, entry); err != nil {
		t.Fatalf("AddWorkspace() error: %v", err)
	}

	if err := deps.repo.RemoveWorkspaceSuppression(ctx, deps.wsID, email, "recipient_resubscribe"); err != nil {
		t.Fatalf("RemoveWorkspaceSuppression() error: %v", err)
	}

	// GetActive now returns NotFound
	_, err := deps.repo.GetActiveWorkspaceSuppression(ctx, deps.wsID, email)
	if err == nil {
		t.Fatal("expected NotFound after removal, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("expected 404 after removal, got: %v", err)
	}

	// Verify removed_at IS NOT NULL and removal_reason in DB
	var removedAt *interface{}
	var removalReason *string
	err = deps.pool.QueryRow(ctx,
		`SELECT removed_at, removal_reason FROM suppression_workspace
		 WHERE workspace_id = $1 AND email = $2`,
		deps.wsID, email,
	).Scan(&removedAt, &removalReason)
	if err != nil {
		t.Fatalf("direct query error: %v", err)
	}
	if removedAt == nil {
		t.Error("expected removed_at to be set")
	}
	if removalReason == nil || *removalReason != "recipient_resubscribe" {
		t.Errorf("removal_reason = %v, want recipient_resubscribe", removalReason)
	}
}

func TestSuppressionRepo_RemoveWorkspaceSuppression_NoActiveRow_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	err := deps.repo.RemoveWorkspaceSuppression(ctx, deps.wsID, "nonexistent-ws@test.com", "reason")
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("expected 404, got: %v", err)
	}
}

func TestSuppressionRepo_AddWorkspace_DoesNotOverrideComplaint(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "complaint-then-unsub@test.com"

	// Insert a complaint suppression first.
	complaint := &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       email,
		Reason:      domain.SuppressionComplaint,
	}
	if err := deps.repo.AddWorkspace(ctx, complaint); err != nil {
		t.Fatalf("initial AddWorkspace(complaint) error: %v", err)
	}

	// Now call AddWorkspace with reason=unsubscribe — must be a no-op.
	unsub := &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       email,
		Reason:      domain.SuppressionUnsubscribe,
	}
	if err := deps.repo.AddWorkspace(ctx, unsub); err != nil {
		t.Fatalf("AddWorkspace(unsubscribe) error: %v", err)
	}

	// Reason in DB must still be complaint.
	var reason string
	err := deps.pool.QueryRow(ctx,
		`SELECT reason FROM suppression_workspace WHERE workspace_id = $1 AND email = $2`,
		deps.wsID, email,
	).Scan(&reason)
	if err != nil {
		t.Fatalf("query reason: %v", err)
	}
	if reason != string(domain.SuppressionComplaint) {
		t.Errorf("reason = %q, want complaint (complaint must not be overwritten by unsubscribe)", reason)
	}
}

func TestSuppressionRepo_AddWorkspace_DoesNotOverrideManual(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	email := "manual-then-unsub@test.com"

	manual := &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       email,
		Reason:      domain.SuppressionManual,
	}
	if err := deps.repo.AddWorkspace(ctx, manual); err != nil {
		t.Fatalf("initial AddWorkspace(manual) error: %v", err)
	}

	unsub := &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       email,
		Reason:      domain.SuppressionUnsubscribe,
	}
	if err := deps.repo.AddWorkspace(ctx, unsub); err != nil {
		t.Fatalf("AddWorkspace(unsubscribe) error: %v", err)
	}

	var reason string
	err := deps.pool.QueryRow(ctx,
		`SELECT reason FROM suppression_workspace WHERE workspace_id = $1 AND email = $2`,
		deps.wsID, email,
	).Scan(&reason)
	if err != nil {
		t.Fatalf("query reason: %v", err)
	}
	if reason != string(domain.SuppressionManual) {
		t.Errorf("reason = %q, want manual (manual must not be overwritten by unsubscribe)", reason)
	}
}

func TestSuppressionRepo_RemoveWorkspaceSuppression_OnlyRemovesUnsubscribe(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)

	// 1. Complaint row: RemoveWorkspaceSuppression must be a no-op (returns NotFound).
	complaintEmail := "complaint-noremove@test.com"
	if err := deps.repo.AddWorkspace(ctx, &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       complaintEmail,
		Reason:      domain.SuppressionComplaint,
	}); err != nil {
		t.Fatalf("AddWorkspace(complaint): %v", err)
	}

	err := deps.repo.RemoveWorkspaceSuppression(ctx, deps.wsID, complaintEmail, "recipient_resubscribe")
	if err == nil {
		t.Fatal("RemoveWorkspaceSuppression on complaint row: expected NotFound, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("expected 404 for complaint row, got: %v", err)
	}

	// Complaint row must remain active.
	sup, err := deps.repo.GetActiveWorkspaceSuppression(ctx, deps.wsID, complaintEmail)
	if err != nil {
		t.Fatalf("complaint row must still be active: %v", err)
	}
	if sup.Reason != domain.SuppressionComplaint {
		t.Errorf("reason = %q, want complaint", sup.Reason)
	}

	// 2. Unsubscribe row: RemoveWorkspaceSuppression must succeed.
	unsubEmail := "unsub-removable@test.com"
	if err := deps.repo.AddWorkspace(ctx, &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       unsubEmail,
		Reason:      domain.SuppressionUnsubscribe,
	}); err != nil {
		t.Fatalf("AddWorkspace(unsubscribe): %v", err)
	}

	if err := deps.repo.RemoveWorkspaceSuppression(ctx, deps.wsID, unsubEmail, "recipient_resubscribe"); err != nil {
		t.Fatalf("RemoveWorkspaceSuppression(unsubscribe) error: %v", err)
	}

	// Unsubscribe row must now be removed.
	_, err = deps.repo.GetActiveWorkspaceSuppression(ctx, deps.wsID, unsubEmail)
	if err == nil {
		t.Fatal("unsubscribe row must be removed, but GetActiveWorkspaceSuppression returned no error")
	}
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("expected 404 after removal, got: %v", err)
	}
}

func TestSuppressionRepo_AddWorkspace_ReactivatesRemovedUnsubscribe(t *testing.T) {
	ctx := context.Background()
	deps := setupSuppressionTestDeps(ctx, t)
	memberRepo := pgadapter.NewMemberRepo(deps.pool)
	_ = createTestMember(ctx, t, memberRepo) // kept for the pattern; not used directly

	email := "reactivate-unsub@test.com"

	// Add unsubscribe, remove it, then re-add — should reactivate.
	if err := deps.repo.AddWorkspace(ctx, &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       email,
		Reason:      domain.SuppressionUnsubscribe,
	}); err != nil {
		t.Fatalf("AddWorkspace: %v", err)
	}
	if err := deps.repo.RemoveWorkspaceSuppression(ctx, deps.wsID, email, "recipient_resubscribe"); err != nil {
		t.Fatalf("RemoveWorkspaceSuppression: %v", err)
	}

	reactivated := &domain.SuppressionWorkspace{
		ID:          uuid.New(),
		WorkspaceID: deps.wsID,
		Email:       email,
		Reason:      domain.SuppressionUnsubscribe,
	}
	if err := deps.repo.AddWorkspace(ctx, reactivated); err != nil {
		t.Fatalf("re-AddWorkspace: %v", err)
	}

	sup, err := deps.repo.GetActiveWorkspaceSuppression(ctx, deps.wsID, email)
	if err != nil {
		t.Fatalf("GetActiveWorkspaceSuppression after reactivation: %v", err)
	}
	if sup.Reason != domain.SuppressionUnsubscribe {
		t.Errorf("reason = %q, want unsubscribe", sup.Reason)
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

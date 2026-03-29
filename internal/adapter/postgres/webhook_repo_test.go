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
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
)

type webhookTestDeps struct {
	pool *pgxpool.Pool
	repo *pgadapter.WebhookRepo
	wsID uuid.UUID
}

func setupWebhookTestDeps(ctx context.Context, t *testing.T) webhookTestDeps {
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

	return webhookTestDeps{
		pool: pool,
		repo: pgadapter.NewWebhookRepo(pool),
		wsID: ws.ID,
	}
}

func newTestWebhook(wsID uuid.UUID) *domain.Webhook {
	return &domain.Webhook{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		URL:         "https://example.com/webhook",
		Secret:      "secret_" + uuid.New().String()[:8],
		Events:      []string{"sent", "delivered"},
		IsActive:    true,
	}
}

func TestWebhookRepo_Create(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	wh := newTestWebhook(deps.wsID)

	if err := deps.repo.Create(ctx, wh); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if wh.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if wh.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestWebhookRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	wh := newTestWebhook(deps.wsID)
	if err := deps.repo.Create(ctx, wh); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := deps.repo.GetByID(ctx, wh.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.URL != wh.URL {
		t.Errorf("want URL %s, got %s", wh.URL, got.URL)
	}
	if len(got.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(got.Events))
	}
	if !got.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestWebhookRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	_, err := deps.repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWebhookRepo_Update(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	wh := newTestWebhook(deps.wsID)
	if err := deps.repo.Create(ctx, wh); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	originalUpdatedAt := wh.UpdatedAt
	wh.URL = "https://updated.example.com/webhook"
	wh.Events = []string{"*"}

	if err := deps.repo.Update(ctx, wh); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	if !wh.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to advance")
	}

	got, err := deps.repo.GetByID(ctx, wh.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.URL != "https://updated.example.com/webhook" {
		t.Errorf("unexpected URL: %s", got.URL)
	}
	if len(got.Events) != 1 || got.Events[0] != "*" {
		t.Errorf("unexpected Events: %v", got.Events)
	}
}

func TestWebhookRepo_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	wh := &domain.Webhook{
		ID:  uuid.New(),
		URL: "https://ghost.example.com",
	}
	err := deps.repo.Update(ctx, wh)
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWebhookRepo_Delete(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	wh := newTestWebhook(deps.wsID)
	if err := deps.repo.Create(ctx, wh); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := deps.repo.Delete(ctx, wh.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err := deps.repo.GetByID(ctx, wh.ID)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestWebhookRepo_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	err := deps.repo.Delete(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestWebhookRepo_ListByWorkspace(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	// Create 5 webhooks
	for i := range 5 {
		wh := newTestWebhook(deps.wsID)
		if err := deps.repo.Create(ctx, wh); err != nil {
			t.Fatalf("Create(%d) error: %v", i, err)
		}
	}

	// Page 1
	result, err := deps.repo.ListByWorkspace(ctx, deps.wsID, port.ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListByWorkspace() error: %v", err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	if !result.HasMore {
		t.Error("expected HasMore to be true")
	}

	// Page 2
	result2, err := deps.repo.ListByWorkspace(ctx, deps.wsID, port.ListOptions{Limit: 3, Cursor: result.NextCursor})
	if err != nil {
		t.Fatalf("ListByWorkspace(page2) error: %v", err)
	}
	if len(result2.Items) != 2 {
		t.Fatalf("expected 2 on page 2, got %d", len(result2.Items))
	}
	if result2.HasMore {
		t.Error("expected HasMore false on last page")
	}
}

func TestWebhookRepo_GetActiveByWorkspace(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	// Active webhook
	wh1 := newTestWebhook(deps.wsID)
	if err := deps.repo.Create(ctx, wh1); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Inactive webhook
	wh2 := newTestWebhook(deps.wsID)
	wh2.IsActive = false
	if err := deps.repo.Create(ctx, wh2); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	active, err := deps.repo.GetActiveByWorkspace(ctx, deps.wsID)
	if err != nil {
		t.Fatalf("GetActiveByWorkspace() error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active webhook, got %d", len(active))
	}
	if active[0].ID != wh1.ID {
		t.Errorf("expected active webhook %s, got %s", wh1.ID, active[0].ID)
	}
}

func TestWebhookRepo_GetActiveByWorkspace_DisabledExcluded(t *testing.T) {
	ctx := context.Background()
	deps := setupWebhookTestDeps(ctx, t)

	// Create webhook then disable it
	wh := newTestWebhook(deps.wsID)
	if err := deps.repo.Create(ctx, wh); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Disable by updating
	now := wh.CreatedAt
	wh.DisabledAt = &now
	if err := deps.repo.Update(ctx, wh); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	active, err := deps.repo.GetActiveByWorkspace(ctx, deps.wsID)
	if err != nil {
		t.Fatalf("GetActiveByWorkspace() error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active after disable, got %d", len(active))
	}
}

func TestWebhookRepo_ListByWorkspace_IsolatesWorkspaces(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	repo := pgadapter.NewWebhookRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	ws1 := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws1-" + uuid.New().String()[:8], Name: "WS1",
	}
	ws2 := &domain.Workspace{
		ID: uuid.New(), TenantID: tenant.ID,
		Code: "ws2-" + uuid.New().String()[:8], Name: "WS2",
	}
	if err := wsRepo.Create(ctx, ws1); err != nil {
		t.Fatalf("creating ws1: %v", err)
	}
	if err := wsRepo.Create(ctx, ws2); err != nil {
		t.Fatalf("creating ws2: %v", err)
	}

	// Create webhook for ws1
	wh := newTestWebhook(ws1.ID)
	if err := repo.Create(ctx, wh); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// List for ws2 should be empty
	result, err := repo.ListByWorkspace(ctx, ws2.ID, port.ListOptions{})
	if err != nil {
		t.Fatalf("ListByWorkspace(ws2) error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 webhooks for ws2, got %d", len(result.Items))
	}
}

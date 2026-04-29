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

func TestAdapterRepo_Create(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &ws.ID,
		Name:               "SES Production",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("encrypted-config"),
		IsDefault:          false,
		RateLimitPerSecond: 14,
	}

	if err := repo.Create(ctx, adapter); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if adapter.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if adapter.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestAdapterRepo_CreateAndGet_SMTP(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	adapter := &domain.Adapter{
		ID:                 uuid.Must(uuid.NewV7()),
		WorkspaceID:        &ws.ID,
		Name:               "SMTP Relay",
		AdapterType:        domain.AdapterTypeSMTP,
		ConfigEncrypted:    []byte(`{"host":"localhost","port":1025,"tls_mode":"none","from_email":"no-reply@example.com"}`),
		IsDefault:          false,
		RateLimitPerSecond: 10,
		ConfigMeta: map[string]string{
			"host":       "localhost",
			"port":       "1025",
			"tls_mode":   "none",
			"from_email": "no-reply@example.com",
		},
	}

	if err := repo.Create(ctx, adapter); err != nil {
		t.Fatalf("Create() SMTP error = %v", err)
	}

	got, err := repo.GetByID(ctx, adapter.ID)
	if err != nil {
		t.Fatalf("GetByID() SMTP error = %v", err)
	}
	if got.AdapterType != domain.AdapterTypeSMTP {
		t.Fatalf("AdapterType = %q, want %q", got.AdapterType, domain.AdapterTypeSMTP)
	}
	if got.ConfigMeta["tls_mode"] != "none" {
		t.Fatalf("ConfigMeta[tls_mode] = %q, want none", got.ConfigMeta["tls_mode"])
	}
}

func TestAdapterRepo_Create_Global(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		Name:               "Global SES",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("encrypted"),
		RateLimitPerSecond: 10,
	}

	if err := repo.Create(ctx, adapter); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
}

func TestAdapterRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		Name:               "GetByID Adapter",
		AdapterType:        domain.AdapterTypeGmail,
		ConfigEncrypted:    []byte("secret-config"),
		IsDefault:          true,
		RateLimitPerSecond: 5,
	}
	if err := repo.Create(ctx, adapter); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByID(ctx, adapter.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Name != "GetByID Adapter" {
		t.Errorf("want name 'GetByID Adapter', got %q", got.Name)
	}
	if got.AdapterType != domain.AdapterTypeGmail {
		t.Errorf("want adapter type gmail, got %q", got.AdapterType)
	}
	if string(got.ConfigEncrypted) != "secret-config" {
		t.Errorf("unexpected config_encrypted")
	}
	if !got.IsDefault {
		t.Error("expected is_default to be true")
	}
	if got.RateLimitPerSecond != 5 {
		t.Errorf("want rate_limit 5, got %d", got.RateLimitPerSecond)
	}
}

func TestAdapterRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	_, err := repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestAdapterRepo_Update(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		Name:               "Original",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("config1"),
		RateLimitPerSecond: 14,
	}
	if err := repo.Create(ctx, adapter); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	originalUpdatedAt := adapter.UpdatedAt
	adapter.Name = "Updated"
	adapter.ConfigEncrypted = []byte("config2")
	adapter.RateLimitPerSecond = 20

	if err := repo.Update(ctx, adapter); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if !adapter.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to advance")
	}

	got, err := repo.GetByID(ctx, adapter.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("want name Updated, got %q", got.Name)
	}
	if string(got.ConfigEncrypted) != "config2" {
		t.Error("expected updated config")
	}
	if got.RateLimitPerSecond != 20 {
		t.Errorf("want rate limit 20, got %d", got.RateLimitPerSecond)
	}
}

func TestAdapterRepo_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	adapter := &domain.Adapter{
		ID:              uuid.New(),
		Name:            "Ghost",
		AdapterType:     domain.AdapterTypeSES,
		ConfigEncrypted: []byte("x"),
	}
	err := repo.Update(ctx, adapter)
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestAdapterRepo_SoftDelete(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		Name:               "To Delete",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("cfg"),
		RateLimitPerSecond: 14,
	}
	if err := repo.Create(ctx, adapter); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.SoftDelete(ctx, adapter.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	_, err := repo.GetByID(ctx, adapter.ID)
	if err == nil {
		t.Fatal("expected not found after soft delete")
	}
}

func TestAdapterRepo_SoftDelete_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	err := repo.SoftDelete(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestAdapterRepo_ListInChain(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	// Global adapter
	globalAdapter := &domain.Adapter{
		ID:                 uuid.New(),
		Name:               "Global",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("g"),
		RateLimitPerSecond: 14,
	}
	if err := repo.Create(ctx, globalAdapter); err != nil {
		t.Fatalf("Create(global) error: %v", err)
	}

	// Workspace adapter
	wsAdapter := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &ws.ID,
		Name:               "WS Adapter",
		AdapterType:        domain.AdapterTypeGmail,
		ConfigEncrypted:    []byte("w"),
		RateLimitPerSecond: 14,
	}
	if err := repo.Create(ctx, wsAdapter); err != nil {
		t.Fatalf("Create(ws) error: %v", err)
	}

	chain := []uuid.NullUUID{
		{UUID: ws.ID, Valid: true},
		{Valid: false}, // global
	}

	adapters, err := repo.ListInChain(ctx, chain)
	if err != nil {
		t.Fatalf("ListInChain() error: %v", err)
	}

	ids := make(map[uuid.UUID]bool)
	for _, a := range adapters {
		ids[a.ID] = true
	}
	if !ids[globalAdapter.ID] {
		t.Error("expected global adapter in chain results")
	}
	if !ids[wsAdapter.ID] {
		t.Error("expected workspace adapter in chain results")
	}
}

func TestAdapterRepo_ListByWorkspace(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	// Create 3 adapters for this workspace
	for i := range 3 {
		a := &domain.Adapter{
			ID:                 uuid.New(),
			WorkspaceID:        &ws.ID,
			Name:               "Adapter",
			AdapterType:        domain.AdapterTypeSES,
			ConfigEncrypted:    []byte("cfg"),
			RateLimitPerSecond: 14,
		}
		if err := repo.Create(ctx, a); err != nil {
			t.Fatalf("Create(%d) error: %v", i, err)
		}
	}

	// Page 1
	result, err := repo.ListByWorkspace(ctx, &ws.ID, port.ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListByWorkspace() error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if !result.HasMore {
		t.Error("expected HasMore to be true")
	}
	if result.NextCursor == "" {
		t.Error("expected non-empty NextCursor")
	}

	// Page 2
	result2, err := repo.ListByWorkspace(ctx, &ws.ID, port.ListOptions{Limit: 2, Cursor: result.NextCursor})
	if err != nil {
		t.Fatalf("ListByWorkspace(page2) error: %v", err)
	}
	if len(result2.Items) != 1 {
		t.Fatalf("expected 1 item on page 2, got %d", len(result2.Items))
	}
	if result2.HasMore {
		t.Error("expected HasMore to be false on last page")
	}
}

func TestAdapterRepo_ListByWorkspace_Global(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewAdapterRepo(pool)

	globalAdapter := &domain.Adapter{
		ID:                 uuid.New(),
		Name:               "Global List",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("g"),
		RateLimitPerSecond: 14,
	}
	if err := repo.Create(ctx, globalAdapter); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	result, err := repo.ListByWorkspace(ctx, nil, port.ListOptions{})
	if err != nil {
		t.Fatalf("ListByWorkspace(nil) error: %v", err)
	}

	found := false
	for _, a := range result.Items {
		if a.ID == globalAdapter.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected global adapter in results")
	}
}

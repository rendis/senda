//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	pgadapter "github.com/senda-app/senda/internal/adapter/postgres"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/apperr"
)

func newTestDomain(wsID *uuid.UUID) *domain.Domain {
	return &domain.Domain{
		ID:                      uuid.New(),
		WorkspaceID:             wsID,
		DomainName:              "test-" + uuid.New().String()[:8] + ".com",
		Status:                  domain.DomainStatusPending,
		DKIMSelector:            "senda",
		DKIMPrivateKeyEncrypted: []byte("encrypted-key"),
		DKIMPublicKey:           "MIGfMA0G...",
		DNSRecords:              []map[string]any{{"type": "TXT", "value": "v=spf1 include:test.com ~all"}},
	}
}

func TestDomainRepo_Create(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	d := newTestDomain(&ws.ID)
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if d.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if d.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestDomainRepo_Create_Global(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	d := newTestDomain(nil)
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
}

func TestDomainRepo_Create_Conflict(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	d1 := newTestDomain(nil)
	if err := repo.Create(ctx, d1); err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	d2 := newTestDomain(nil)
	d2.DomainName = d1.DomainName // same domain name in global scope
	err := repo.Create(ctx, d2)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestDomainRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	d := newTestDomain(nil)
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByID(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.DomainName != d.DomainName {
		t.Errorf("want domain_name %q, got %q", d.DomainName, got.DomainName)
	}
	if got.Status != domain.DomainStatusPending {
		t.Errorf("want status pending, got %q", got.Status)
	}
	if got.DKIMSelector != "senda" {
		t.Errorf("want dkim_selector senda, got %q", got.DKIMSelector)
	}
	if len(got.DNSRecords) == 0 {
		t.Error("expected dns_records to be non-empty")
	}
}

func TestDomainRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	_, err := repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestDomainRepo_Update(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	d := newTestDomain(nil)
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	originalUpdatedAt := d.UpdatedAt
	d.Status = domain.DomainStatusVerified
	now := time.Now()
	d.VerifiedAt = &now
	errMsg := "some error"
	d.LastError = &errMsg

	if err := repo.Update(ctx, d); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if !d.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to advance")
	}

	got, err := repo.GetByID(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Status != domain.DomainStatusVerified {
		t.Errorf("want status verified, got %q", got.Status)
	}
	if got.VerifiedAt == nil {
		t.Error("expected VerifiedAt to be set")
	}
	if got.LastError == nil || *got.LastError != errMsg {
		t.Errorf("expected last_error %q, got %v", errMsg, got.LastError)
	}
}

func TestDomainRepo_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	d := newTestDomain(nil)
	d.ID = uuid.New()
	err := repo.Update(ctx, d)
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestDomainRepo_SoftDelete(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	d := newTestDomain(nil)
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.SoftDelete(ctx, d.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	_, err := repo.GetByID(ctx, d.ID)
	if err == nil {
		t.Fatal("expected not found after soft delete")
	}
}

func TestDomainRepo_SoftDelete_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	err := repo.SoftDelete(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestDomainRepo_ListInChain(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	globalDomain := newTestDomain(nil)
	if err := repo.Create(ctx, globalDomain); err != nil {
		t.Fatalf("Create(global) error: %v", err)
	}

	wsDomain := newTestDomain(&ws.ID)
	if err := repo.Create(ctx, wsDomain); err != nil {
		t.Fatalf("Create(ws) error: %v", err)
	}

	chain := []uuid.NullUUID{
		{UUID: ws.ID, Valid: true},
		{Valid: false},
	}

	domains, err := repo.ListInChain(ctx, chain)
	if err != nil {
		t.Fatalf("ListInChain() error: %v", err)
	}

	ids := make(map[uuid.UUID]bool)
	for _, d := range domains {
		ids[d.ID] = true
	}
	if !ids[globalDomain.ID] {
		t.Error("expected global domain in chain results")
	}
	if !ids[wsDomain.ID] {
		t.Error("expected workspace domain in chain results")
	}
}

func TestDomainRepo_ListByWorkspace(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	for range 3 {
		d := newTestDomain(&ws.ID)
		if err := repo.Create(ctx, d); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}

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

func TestDomainRepo_GetPendingVerifications(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewDomainRepo(pool)

	// Pending domain with no next_check_at (should be returned first)
	d1 := newTestDomain(nil)
	d1.Status = domain.DomainStatusPending
	if err := repo.Create(ctx, d1); err != nil {
		t.Fatalf("Create(d1) error: %v", err)
	}

	// Error domain with next_check_at in the past
	d2 := newTestDomain(nil)
	d2.Status = domain.DomainStatusError
	past := time.Now().Add(-1 * time.Hour)
	d2.NextCheckAt = &past
	if err := repo.Create(ctx, d2); err != nil {
		t.Fatalf("Create(d2) error: %v", err)
	}

	// Verified domain (should NOT be returned)
	d3 := newTestDomain(nil)
	d3.Status = domain.DomainStatusVerified
	if err := repo.Create(ctx, d3); err != nil {
		t.Fatalf("Create(d3) error: %v", err)
	}

	// Pending domain with next_check_at in the future (should NOT be returned)
	d4 := newTestDomain(nil)
	d4.Status = domain.DomainStatusPending
	future := time.Now().Add(1 * time.Hour)
	d4.NextCheckAt = &future
	if err := repo.Create(ctx, d4); err != nil {
		t.Fatalf("Create(d4) error: %v", err)
	}

	domains, err := repo.GetPendingVerifications(ctx, 10)
	if err != nil {
		t.Fatalf("GetPendingVerifications() error: %v", err)
	}

	ids := make(map[uuid.UUID]bool)
	for _, d := range domains {
		ids[d.ID] = true
	}

	if !ids[d1.ID] {
		t.Error("expected d1 (pending, no next_check) in results")
	}
	if !ids[d2.ID] {
		t.Error("expected d2 (error, past next_check) in results")
	}
	if ids[d3.ID] {
		t.Error("d3 (verified) should not be in results")
	}
	if ids[d4.ID] {
		t.Error("d4 (pending, future next_check) should not be in results")
	}
}

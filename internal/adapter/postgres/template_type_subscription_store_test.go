//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

// ttsFullDeps holds the store under test, a seeded workspace ID, and a
// TemplateRepo so individual tests can create template types without raw SQL.
type ttsFullDeps struct {
	store  *pgadapter.TemplateTypeSubscriptionStore
	wsID   uuid.UUID
	ttRepo *pgadapter.TemplateRepo
}

func setupTTSFull(ctx context.Context, t *testing.T) ttsFullDeps {
	t.Helper()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	return ttsFullDeps{
		store:  pgadapter.NewTemplateTypeSubscriptionStore(pool),
		wsID:   ws.ID,
		ttRepo: pgadapter.NewTemplateRepo(pool),
	}
}

// newTemplateType seeds a template_type row and returns the domain object.
func (d ttsFullDeps) newTemplateType(ctx context.Context, t *testing.T) *domain.TemplateType {
	t.Helper()
	tt := &domain.TemplateType{
		ID:             uuid.Must(uuid.NewV7()),
		Slug:           "tt-" + uuid.New().String()[:8],
		Name:           "Test Type",
		VariableSchema: map[string]any{},
	}
	if err := d.ttRepo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}
	return tt
}

// ---- Tests ----

func TestTTSStore_Upsert_InsertsThenUpdates(t *testing.T) {
	ctx := context.Background()
	deps := setupTTSFull(ctx, t)
	tt := deps.newTemplateType(ctx, t)

	// First upsert: insert with subscribed=false.
	sub := &domain.TemplateTypeSubscription{
		ID:             uuid.Must(uuid.NewV7()),
		WorkspaceID:    deps.wsID,
		TemplateTypeID: tt.ID,
		Email:          "user@example.com",
		Subscribed:     false,
		Source:         domain.SubscriptionSourceRecipientOptout,
	}
	if err := deps.store.Upsert(ctx, sub); err != nil {
		t.Fatalf("Upsert (insert) error: %v", err)
	}

	// GetState: must exist, subscribed=false.
	got, err := deps.store.GetState(ctx, deps.wsID, tt.ID, sub.Email)
	if err != nil {
		t.Fatalf("GetState after insert error: %v", err)
	}
	if got.Subscribed {
		t.Error("expected subscribed=false after insert")
	}
	if got.Source != domain.SubscriptionSourceRecipientOptout {
		t.Errorf("expected source=%s, got %s", domain.SubscriptionSourceRecipientOptout, got.Source)
	}
	firstID := got.ID

	// Second upsert: same key, subscribed=true.
	sub2 := &domain.TemplateTypeSubscription{
		ID:             uuid.Must(uuid.NewV7()), // new UUID — must NOT create a second row
		WorkspaceID:    deps.wsID,
		TemplateTypeID: tt.ID,
		Email:          sub.Email,
		Subscribed:     true,
		Source:         domain.SubscriptionSourceRecipientOptin,
	}
	if err := deps.store.Upsert(ctx, sub2); err != nil {
		t.Fatalf("Upsert (update) error: %v", err)
	}

	// GetState: same ID (UPDATE, not INSERT), subscribed=true.
	got2, err := deps.store.GetState(ctx, deps.wsID, tt.ID, sub.Email)
	if err != nil {
		t.Fatalf("GetState after update error: %v", err)
	}
	if !got2.Subscribed {
		t.Error("expected subscribed=true after update")
	}
	if got2.ID != firstID {
		t.Errorf("expected same row ID %s, got %s (ON CONFLICT created a new row)", firstID, got2.ID)
	}
	if got2.Source != domain.SubscriptionSourceRecipientOptin {
		t.Errorf("expected source=%s, got %s", domain.SubscriptionSourceRecipientOptin, got2.Source)
	}
}

func TestTTSStore_GetState_NotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupTTSFull(ctx, t)
	tt := deps.newTemplateType(ctx, t)

	_, err := deps.store.GetState(ctx, deps.wsID, tt.ID, "nobody@example.com")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404 AppError, got: %v", err)
	}
}

func TestTTSStore_ListOptOutsForRecipient(t *testing.T) {
	ctx := context.Background()
	deps := setupTTSFull(ctx, t)

	tt1 := deps.newTemplateType(ctx, t)
	tt2 := deps.newTemplateType(ctx, t)

	email1 := "email1@example.com"
	email2 := "email2@example.com"

	entries := []*domain.TemplateTypeSubscription{
		// email1: opted-out of tt1
		{ID: uuid.Must(uuid.NewV7()), WorkspaceID: deps.wsID, TemplateTypeID: tt1.ID, Email: email1, Subscribed: false, Source: domain.SubscriptionSourceRecipientOptout},
		// email1: opted-in to tt2
		{ID: uuid.Must(uuid.NewV7()), WorkspaceID: deps.wsID, TemplateTypeID: tt2.ID, Email: email1, Subscribed: true, Source: domain.SubscriptionSourceRecipientOptin},
		// email2: opted-out of tt1 (must NOT appear in email1 results)
		{ID: uuid.Must(uuid.NewV7()), WorkspaceID: deps.wsID, TemplateTypeID: tt1.ID, Email: email2, Subscribed: false, Source: domain.SubscriptionSourceRecipientOptout},
	}
	for _, e := range entries {
		if err := deps.store.Upsert(ctx, e); err != nil {
			t.Fatalf("Upsert error: %v", err)
		}
	}

	rows, err := deps.store.ListOptOutsForRecipient(ctx, deps.wsID, email1)
	if err != nil {
		t.Fatalf("ListOptOutsForRecipient error: %v", err)
	}

	// Must return exactly 2 rows (both for email1, regardless of subscribed state).
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for email1, got %d", len(rows))
	}

	// Verify both template type IDs are present.
	ttIDs := map[uuid.UUID]bool{}
	for _, r := range rows {
		if r.Email != email1 {
			t.Errorf("unexpected email %s in result", r.Email)
		}
		ttIDs[r.TemplateTypeID] = true
	}
	if !ttIDs[tt1.ID] {
		t.Error("expected tt1 in results")
	}
	if !ttIDs[tt2.ID] {
		t.Error("expected tt2 in results")
	}
}

func TestTTSStore_BatchCheckOptOut(t *testing.T) {
	ctx := context.Background()
	deps := setupTTSFull(ctx, t)
	tt := deps.newTemplateType(ctx, t)

	// out@x.com is explicitly opted-out.
	if err := deps.store.Upsert(ctx, &domain.TemplateTypeSubscription{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: deps.wsID, TemplateTypeID: tt.ID,
		Email: "out@x.com", Subscribed: false, Source: domain.SubscriptionSourceRecipientOptout,
	}); err != nil {
		t.Fatalf("Upsert out@x.com error: %v", err)
	}

	// in@x.com is explicitly opted-in.
	if err := deps.store.Upsert(ctx, &domain.TemplateTypeSubscription{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: deps.wsID, TemplateTypeID: tt.ID,
		Email: "in@x.com", Subscribed: true, Source: domain.SubscriptionSourceRecipientOptin,
	}); err != nil {
		t.Fatalf("Upsert in@x.com error: %v", err)
	}

	// never@x.com has no row at all.

	result, err := deps.store.BatchCheckOptOut(ctx, deps.wsID, tt.ID, []string{"out@x.com", "in@x.com", "never@x.com"})
	if err != nil {
		t.Fatalf("BatchCheckOptOut error: %v", err)
	}

	if _, ok := result["out@x.com"]; !ok {
		t.Error("expected out@x.com in result")
	}
	if _, ok := result["in@x.com"]; ok {
		t.Error("did not expect in@x.com in result (subscribed=true)")
	}
	if _, ok := result["never@x.com"]; ok {
		t.Error("did not expect never@x.com in result (no row)")
	}
	if len(result) != 1 {
		t.Errorf("expected exactly 1 opt-out, got %d", len(result))
	}
}

func TestTTSStore_BatchCheckOptOut_EmptyEmails(t *testing.T) {
	ctx := context.Background()
	deps := setupTTSFull(ctx, t)
	tt := deps.newTemplateType(ctx, t)

	result, err := deps.store.BatchCheckOptOut(ctx, deps.wsID, tt.ID, []string{})
	if err != nil {
		t.Fatalf("BatchCheckOptOut (empty) error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil map for empty input")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

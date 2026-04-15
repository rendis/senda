//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// emailTestDeps holds common dependencies for email tests.
type emailTestDeps struct {
	pool     *pgxpool.Pool
	repo     *pgadapter.EmailRepo
	tenantID uuid.UUID
	wsID     uuid.UUID
}

func setupEmailTestDeps(ctx context.Context, t *testing.T) emailTestDeps {
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

	return emailTestDeps{
		pool:     pool,
		repo:     pgadapter.NewEmailRepo(pool),
		tenantID: tenant.ID,
		wsID:     ws.ID,
	}
}

func newTestEmail(tenantID, wsID uuid.UUID) *domain.Email {
	return &domain.Email{
		ID:                uuid.New(),
		TrackingID:        uuid.New().String()[:32],
		WorkspaceID:       wsID,
		TenantID:          tenantID,
		TemplateID:        uuid.New(),
		TemplateVersionID: uuid.New(),
		TemplateTypeSlug:  "welcome",
		TemplateRef:       "acme:welcome",
		RecipientEmail:    "recipient@test.com",
		FromEmail:         "sender@test.com",
		FromName:          "Sender",
		SubjectRendered:   "Welcome!",
		Status:            domain.StatusQueued,
		AdapterID:         uuid.New(),
		RetryCount:        0,
		MaxRetries:        3,
	}
}

func TestEmailRepo_Create(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	email := newTestEmail(deps.tenantID, deps.wsID)
	email.CC = []string{"cc@test.com"}
	email.BCC = []string{"bcc@test.com"}

	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if email.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if email.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestEmailRepo_GetByTrackingID(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	email := newTestEmail(deps.tenantID, deps.wsID)
	senderIdentityID := uuid.New()
	email.CC = []string{"cc@test.com"}
	email.BCC = []string{"bcc1@test.com", "bcc2@test.com"}
	extID := "ext-123"
	email.ExternalID = &extID
	email.SenderIdentityID = &senderIdentityID

	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := deps.repo.GetByTrackingID(ctx, email.TrackingID)
	if err != nil {
		t.Fatalf("GetByTrackingID() error: %v", err)
	}
	if got.ID != email.ID {
		t.Errorf("want ID %s, got %s", email.ID, got.ID)
	}
	if len(got.CC) != 1 || got.CC[0] != "cc@test.com" {
		t.Errorf("unexpected CC: %v", got.CC)
	}
	if len(got.BCC) != 2 {
		t.Errorf("expected 2 BCC entries, got %d", len(got.BCC))
	}
	if got.ExternalID == nil || *got.ExternalID != "ext-123" {
		t.Errorf("expected ExternalID ext-123, got %v", got.ExternalID)
	}
	if got.SenderIdentityID == nil || *got.SenderIdentityID != senderIdentityID {
		t.Errorf("expected SenderIdentityID %s, got %v", senderIdentityID, got.SenderIdentityID)
	}
}

func TestEmailRepo_GetByTrackingID_ColdPayloadDeferred(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	email := newTestEmail(deps.tenantID, deps.wsID)
	email.BodyMJML = "<mj-text>Hello {{ name }}</mj-text>"
	email.VariablesSnapshot = map[string]any{"name": "Ana"}
	email.InjectorsSnapshot = map[string]map[string]any{"brand": {"name": "Acme"}}

	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	hot, err := deps.repo.GetByTrackingID(ctx, email.TrackingID)
	if err != nil {
		t.Fatalf("GetByTrackingID() error: %v", err)
	}
	if hot.BodyMJML != "" {
		t.Fatalf("expected hot row without BodyMJML, got %q", hot.BodyMJML)
	}
	if hot.VariablesSnapshot != nil {
		t.Fatalf("expected hot row without VariablesSnapshot, got %#v", hot.VariablesSnapshot)
	}
	if hot.InjectorsSnapshot != nil {
		t.Fatalf("expected hot row without InjectorsSnapshot, got %#v", hot.InjectorsSnapshot)
	}

	payload, err := deps.repo.GetPayload(ctx, email.ID)
	if err != nil {
		t.Fatalf("GetPayload() error: %v", err)
	}
	if payload.BodyMJML != email.BodyMJML {
		t.Fatalf("expected payload BodyMJML %q, got %q", email.BodyMJML, payload.BodyMJML)
	}
	if got := payload.VariablesSnapshot["name"]; got != "Ana" {
		t.Fatalf("expected payload variable name=Ana, got %#v", got)
	}
	if got := payload.InjectorsSnapshot["brand"]["name"]; got != "Acme" {
		t.Fatalf("expected payload injector brand.name=Acme, got %#v", got)
	}
}

func TestEmailRepo_GetByTrackingID_NotFound(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	_, err := deps.repo.GetByTrackingID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestEmailRepo_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	email := newTestEmail(deps.tenantID, deps.wsID)
	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := deps.repo.UpdateStatus(ctx, email.ID, domain.StatusSent, domain.StatusQueued); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	got, err := deps.repo.GetByTrackingID(ctx, email.TrackingID)
	if err != nil {
		t.Fatalf("GetByTrackingID() error: %v", err)
	}
	if got.Status != domain.StatusSent {
		t.Errorf("expected status sent, got %s", got.Status)
	}
}

func TestEmailRepo_UpdateRetry(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	email := newTestEmail(deps.tenantID, deps.wsID)
	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	nextRetry := time.Now().Add(5 * time.Minute)
	if err := deps.repo.UpdateRetry(ctx, email.ID, 1, &nextRetry); err != nil {
		t.Fatalf("UpdateRetry() error: %v", err)
	}

	got, err := deps.repo.GetByTrackingID(ctx, email.TrackingID)
	if err != nil {
		t.Fatalf("GetByTrackingID() error: %v", err)
	}
	if got.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", got.RetryCount)
	}
	if got.NextRetryAt == nil {
		t.Error("expected NextRetryAt to be set")
	}
}

func TestEmailRepo_AddEvent_GetEvents(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	email := newTestEmail(deps.tenantID, deps.wsID)
	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	event := &domain.EmailEvent{
		ID:         uuid.New(),
		EmailID:    email.ID,
		EventType:  domain.EventTypeSent,
		OccurredAt: time.Now().UTC(),
		Metadata:   map[string]any{"provider": "ses"},
	}

	if err := deps.repo.AddEvent(ctx, event); err != nil {
		t.Fatalf("AddEvent() error: %v", err)
	}

	if event.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set on event")
	}

	events, err := deps.repo.GetEvents(ctx, email.ID)
	if err != nil {
		t.Fatalf("GetEvents() error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != domain.EventTypeSent {
		t.Errorf("expected event type sent, got %s", events[0].EventType)
	}
}

func TestEmailRepo_QueryByExternalID(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	extID := "ext-query-" + uuid.New().String()[:8]

	// Create 3 emails with same external ID
	for range 3 {
		email := newTestEmail(deps.tenantID, deps.wsID)
		email.ExternalID = &extID
		if err := deps.repo.Create(ctx, email); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}

	emails, cursor, err := deps.repo.QueryByExternalID(ctx, deps.wsID, extID, "", 10)
	if err != nil {
		t.Fatalf("QueryByExternalID() error: %v", err)
	}
	if len(emails) != 3 {
		t.Fatalf("expected 3 emails, got %d", len(emails))
	}
	if cursor != "" {
		t.Error("expected empty cursor (no more pages)")
	}
}

func TestEmailRepo_QueryByRecipient(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	recipient := "specific@recipient.com"

	email := newTestEmail(deps.tenantID, deps.wsID)
	email.RecipientEmail = recipient
	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	emails, _, err := deps.repo.QueryByRecipient(ctx, deps.wsID, recipient, "", 10)
	if err != nil {
		t.Fatalf("QueryByRecipient() error: %v", err)
	}
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	if emails[0].RecipientEmail != recipient {
		t.Errorf("expected recipient %s, got %s", recipient, emails[0].RecipientEmail)
	}
}

func TestEmailRepo_QueryByWorkspace(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	// Create emails with different statuses
	for _, status := range []domain.EmailStatus{domain.StatusQueued, domain.StatusSent, domain.StatusSent} {
		email := newTestEmail(deps.tenantID, deps.wsID)
		email.Status = status
		if err := deps.repo.Create(ctx, email); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}

	// Query all
	all, _, err := deps.repo.QueryByWorkspace(ctx, deps.wsID, port.EmailFilters{}, "", 10)
	if err != nil {
		t.Fatalf("QueryByWorkspace() error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 emails, got %d", len(all))
	}

	// Query by status
	sentStatus := domain.StatusSent
	sent, _, err := deps.repo.QueryByWorkspace(ctx, deps.wsID, port.EmailFilters{Status: &sentStatus}, "", 10)
	if err != nil {
		t.Fatalf("QueryByWorkspace(status=sent) error: %v", err)
	}
	if len(sent) != 2 {
		t.Fatalf("expected 2 sent emails, got %d", len(sent))
	}
}

func TestEmailRepo_QueryByWorkspace_Pagination(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	// Create 5 emails
	for range 5 {
		email := newTestEmail(deps.tenantID, deps.wsID)
		if err := deps.repo.Create(ctx, email); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		// Small sleep to ensure distinct created_at values
		time.Sleep(time.Millisecond)
	}

	// Page 1
	page1, cursor1, err := deps.repo.QueryByWorkspace(ctx, deps.wsID, port.EmailFilters{}, "", 3)
	if err != nil {
		t.Fatalf("QueryByWorkspace(page1) error: %v", err)
	}
	if len(page1) != 3 {
		t.Fatalf("expected 3 on page 1, got %d", len(page1))
	}
	if cursor1 == "" {
		t.Fatal("expected non-empty cursor for page 1")
	}

	// Page 2
	page2, cursor2, err := deps.repo.QueryByWorkspace(ctx, deps.wsID, port.EmailFilters{}, cursor1, 3)
	if err != nil {
		t.Fatalf("QueryByWorkspace(page2) error: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 on page 2, got %d", len(page2))
	}
	if cursor2 != "" {
		t.Error("expected empty cursor for last page")
	}

	// Verify no overlap
	seen := make(map[uuid.UUID]bool)
	for _, e := range page1 {
		seen[e.ID] = true
	}
	for _, e := range page2 {
		if seen[e.ID] {
			t.Errorf("email %s appears on both pages", e.ID)
		}
	}
}

func TestEmailRepo_QueryByWorkspace_InvalidCursor(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	_, _, err := deps.repo.QueryByWorkspace(ctx, deps.wsID, port.EmailFilters{}, "not-a-valid-cursor", 10)
	if err == nil {
		t.Fatal("expected error for invalid cursor")
	}
	if !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestEmailRepo_QueryByExternalIDGlobal(t *testing.T) {
	ctx := context.Background()
	deps := setupEmailTestDeps(ctx, t)

	extID := "global-ext-" + uuid.New().String()[:8]

	email := newTestEmail(deps.tenantID, deps.wsID)
	email.ExternalID = &extID
	if err := deps.repo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	emails, _, err := deps.repo.QueryByExternalIDGlobal(ctx, extID, "", 10)
	if err != nil {
		t.Fatalf("QueryByExternalIDGlobal() error: %v", err)
	}
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
}

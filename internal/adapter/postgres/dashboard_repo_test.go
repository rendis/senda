//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// dashboardTestDeps holds common dependencies for dashboard tests.
type dashboardTestDeps struct {
	pool      *pgxpool.Pool
	dashRepo  *pgadapter.DashboardRepo
	emailRepo *pgadapter.EmailRepo
	tenantID  uuid.UUID
	wsID      uuid.UUID
}

func setupDashboardTestDeps(ctx context.Context, t *testing.T) dashboardTestDeps {
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

	return dashboardTestDeps{
		pool:      pool,
		dashRepo:  pgadapter.NewDashboardRepo(pool),
		emailRepo: pgadapter.NewEmailRepo(pool),
		tenantID:  tenant.ID,
		wsID:      ws.ID,
	}
}

func newDashboardTestEmail(tenantID, wsID uuid.UUID, status domain.EmailStatus) *domain.Email {
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
		Status:            status,
		AdapterID:         uuid.New(),
		RetryCount:        0,
		MaxRetries:        3,
	}
}

func TestDashboardRepo_GetTotals_MixedStatuses(t *testing.T) {
	ctx := context.Background()
	deps := setupDashboardTestDeps(ctx, t)

	// Create emails with various statuses.
	statuses := []domain.EmailStatus{
		domain.StatusSent,
		domain.StatusDelivered,
		domain.StatusDelivered,
		domain.StatusBounced,
		domain.StatusComplained,
		domain.StatusFailed,
		domain.StatusQueued, // should not count in any total
	}
	for _, s := range statuses {
		email := newDashboardTestEmail(deps.tenantID, deps.wsID, s)
		if err := deps.emailRepo.Create(ctx, email); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}

	params := port.DashboardStatsParams{
		WorkspaceID: &deps.wsID,
		Since:       time.Now().Add(-1 * time.Hour),
		Until:       time.Now().Add(1 * time.Hour),
	}

	totals, err := deps.dashRepo.GetTotals(ctx, params)
	if err != nil {
		t.Fatalf("GetTotals() error: %v", err)
	}

	// sent = sent + delivered + opened = 1 + 2 + 0 = 3
	if totals.Sent != 3 {
		t.Errorf("expected Sent=3, got %d", totals.Sent)
	}
	// delivered = delivered + opened = 2 + 0 = 2
	if totals.Delivered != 2 {
		t.Errorf("expected Delivered=2, got %d", totals.Delivered)
	}
	if totals.Bounced != 1 {
		t.Errorf("expected Bounced=1, got %d", totals.Bounced)
	}
	if totals.Complained != 1 {
		t.Errorf("expected Complained=1, got %d", totals.Complained)
	}
	if totals.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", totals.Failed)
	}
}

func TestDashboardRepo_GetTimeSeries_DailyBuckets(t *testing.T) {
	ctx := context.Background()
	deps := setupDashboardTestDeps(ctx, t)

	// Create 3 emails (all created today).
	for _, s := range []domain.EmailStatus{domain.StatusSent, domain.StatusDelivered, domain.StatusFailed} {
		email := newDashboardTestEmail(deps.tenantID, deps.wsID, s)
		if err := deps.emailRepo.Create(ctx, email); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}

	params := port.DashboardStatsParams{
		WorkspaceID: &deps.wsID,
		Since:       time.Now().Add(-24 * time.Hour),
		Until:       time.Now().Add(24 * time.Hour),
	}

	series, err := deps.dashRepo.GetTimeSeries(ctx, params)
	if err != nil {
		t.Fatalf("GetTimeSeries() error: %v", err)
	}

	if len(series) != 1 {
		t.Fatalf("expected 1 daily bucket, got %d", len(series))
	}

	pt := series[0]
	// sent = sent + delivered = 1 + 1 = 2
	if pt.Sent != 2 {
		t.Errorf("expected Sent=2, got %d", pt.Sent)
	}
	if pt.Delivered != 1 {
		t.Errorf("expected Delivered=1, got %d", pt.Delivered)
	}
	if pt.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", pt.Failed)
	}
}

func TestDashboardRepo_GetRecentEmails_LimitedAndOrdered(t *testing.T) {
	ctx := context.Background()
	deps := setupDashboardTestDeps(ctx, t)

	// Create 5 emails.
	for range 5 {
		email := newDashboardTestEmail(deps.tenantID, deps.wsID, domain.StatusSent)
		if err := deps.emailRepo.Create(ctx, email); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		time.Sleep(time.Millisecond)
	}

	params := port.DashboardStatsParams{
		WorkspaceID: &deps.wsID,
		Since:       time.Now().Add(-1 * time.Hour),
		Until:       time.Now().Add(1 * time.Hour),
	}

	emails, err := deps.dashRepo.GetRecentEmails(ctx, params, 3)
	if err != nil {
		t.Fatalf("GetRecentEmails() error: %v", err)
	}
	if len(emails) != 3 {
		t.Fatalf("expected 3 emails, got %d", len(emails))
	}

	// Verify descending order by created_at.
	for i := 1; i < len(emails); i++ {
		if emails[i].CreatedAt.After(emails[i-1].CreatedAt) {
			t.Errorf("emails not in descending order: [%d]=%s > [%d]=%s",
				i, emails[i].CreatedAt, i-1, emails[i-1].CreatedAt)
		}
	}
}

func TestDashboardRepo_ScopeFiltering_WorkspaceVsGlobal(t *testing.T) {
	ctx := context.Background()
	deps := setupDashboardTestDeps(ctx, t)

	// Create a second workspace.
	wsRepo := pgadapter.NewWorkspaceRepo(deps.pool)
	ws2 := &domain.Workspace{
		ID: uuid.New(), TenantID: deps.tenantID,
		Code: "ws2-" + uuid.New().String()[:8], Name: "WS2",
	}
	if err := wsRepo.Create(ctx, ws2); err != nil {
		t.Fatalf("creating workspace 2: %v", err)
	}

	// Create 2 emails in ws1.
	for range 2 {
		email := newDashboardTestEmail(deps.tenantID, deps.wsID, domain.StatusSent)
		if err := deps.emailRepo.Create(ctx, email); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}
	// Create 1 email in ws2.
	email := newDashboardTestEmail(deps.tenantID, ws2.ID, domain.StatusSent)
	if err := deps.emailRepo.Create(ctx, email); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	timeRange := port.DashboardStatsParams{
		Since: time.Now().Add(-1 * time.Hour),
		Until: time.Now().Add(1 * time.Hour),
	}

	// Workspace-scoped: should see only ws1 emails.
	ws1Params := timeRange
	ws1Params.WorkspaceID = &deps.wsID
	totals, err := deps.dashRepo.GetTotals(ctx, ws1Params)
	if err != nil {
		t.Fatalf("GetTotals(ws1) error: %v", err)
	}
	if totals.Sent != 2 {
		t.Errorf("workspace scope: expected Sent=2, got %d", totals.Sent)
	}

	// Global: should see all 3 emails.
	globalParams := timeRange
	totals, err = deps.dashRepo.GetTotals(ctx, globalParams)
	if err != nil {
		t.Fatalf("GetTotals(global) error: %v", err)
	}
	if totals.Sent != 3 {
		t.Errorf("global scope: expected Sent=3, got %d", totals.Sent)
	}

	// Tenant-scoped: should see all 3 emails (both workspaces belong to same tenant).
	tenantParams := timeRange
	tenantParams.TenantID = &deps.tenantID
	totals, err = deps.dashRepo.GetTotals(ctx, tenantParams)
	if err != nil {
		t.Fatalf("GetTotals(tenant) error: %v", err)
	}
	if totals.Sent != 3 {
		t.Errorf("tenant scope: expected Sent=3, got %d", totals.Sent)
	}
}

func TestDashboardRepo_GetTotalsByAdapter_SplitsSharedSESBySenderIdentityAndFromEmail(t *testing.T) {
	ctx := context.Background()
	deps := setupDashboardTestDeps(ctx, t)

	sharedAdapterID := uuid.New()
	senderA := uuid.New()
	senderB := uuid.New()

	emailA := newDashboardTestEmail(deps.tenantID, deps.wsID, domain.StatusDelivered)
	emailA.AdapterID = sharedAdapterID
	emailA.FromEmail = "a@shared-mail.test"
	emailA.SenderIdentityID = &senderA
	if err := deps.emailRepo.Create(ctx, emailA); err != nil {
		t.Fatalf("Create(emailA) error: %v", err)
	}

	emailB := newDashboardTestEmail(deps.tenantID, deps.wsID, domain.StatusSent)
	emailB.AdapterID = sharedAdapterID
	emailB.FromEmail = "b@shared-mail.test"
	emailB.SenderIdentityID = &senderB
	if err := deps.emailRepo.Create(ctx, emailB); err != nil {
		t.Fatalf("Create(emailB) error: %v", err)
	}

	params := port.DashboardStatsParams{
		WorkspaceID: &deps.wsID,
		Since:       time.Now().Add(-1 * time.Hour),
		Until:       time.Now().Add(1 * time.Hour),
	}

	rows, err := deps.dashRepo.GetTotalsByAdapter(ctx, params)
	if err != nil {
		t.Fatalf("GetTotalsByAdapter() error: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	got := map[string]port.DashboardAdapterTotals{}
	for _, row := range rows {
		got[row.FromEmail] = row
	}

	rowA, ok := got["a@shared-mail.test"]
	if !ok {
		t.Fatalf("missing row for sender a")
	}
	if rowA.SenderIdentityID == nil || *rowA.SenderIdentityID != senderA {
		t.Fatalf("expected sender identity %s for rowA, got %v", senderA, rowA.SenderIdentityID)
	}
	if rowA.Totals.Delivered != 1 || rowA.Totals.Sent != 1 {
		t.Fatalf("expected rowA delivered=1 sent=1, got delivered=%d sent=%d", rowA.Totals.Delivered, rowA.Totals.Sent)
	}

	rowB, ok := got["b@shared-mail.test"]
	if !ok {
		t.Fatalf("missing row for sender b")
	}
	if rowB.SenderIdentityID == nil || *rowB.SenderIdentityID != senderB {
		t.Fatalf("expected sender identity %s for rowB, got %v", senderB, rowB.SenderIdentityID)
	}
	if rowB.Totals.Delivered != 0 || rowB.Totals.Sent != 1 {
		t.Fatalf("expected rowB delivered=0 sent=1, got delivered=%d sent=%d", rowB.Totals.Delivered, rowB.Totals.Sent)
	}
}

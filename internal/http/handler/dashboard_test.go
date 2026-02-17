package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

// --- Mock DashboardStore ---

type mockDashboardStore struct {
	getTotalsFn      func(ctx context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error)
	getTimeSeriesFn  func(ctx context.Context, p port.DashboardStatsParams) ([]port.DashboardTimePoint, error)
	getRecentEmailsFn func(ctx context.Context, p port.DashboardStatsParams, limit int) ([]port.DashboardRecentEmail, error)
}

func (m *mockDashboardStore) GetTotals(ctx context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error) {
	if m.getTotalsFn != nil {
		return m.getTotalsFn(ctx, p)
	}
	return &port.DashboardTotals{}, nil
}

func (m *mockDashboardStore) GetTimeSeries(ctx context.Context, p port.DashboardStatsParams) ([]port.DashboardTimePoint, error) {
	if m.getTimeSeriesFn != nil {
		return m.getTimeSeriesFn(ctx, p)
	}
	return nil, nil
}

func (m *mockDashboardStore) GetRecentEmails(ctx context.Context, p port.DashboardStatsParams, limit int) ([]port.DashboardRecentEmail, error) {
	if m.getRecentEmailsFn != nil {
		return m.getRecentEmailsFn(ctx, p, limit)
	}
	return nil, nil
}

// --- Helpers ---

func setupDashboardTest(ds port.DashboardStore, als port.AuditLogStore, ts port.TenantStore, ws port.WorkspaceStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewDashboardHandler(ds, als, ts, ws)

	wsBase := "/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code"
	e.GET(wsBase+"/dashboard-stats", h.Stats)
	e.GET("/api/v1/manage/tenants/:tenant_code/dashboard-stats", h.StatsTenant)
	e.GET("/api/v1/manage/global/dashboard-stats", h.StatsGlobal)

	return e
}

func defaultMockDashboardStore() *mockDashboardStore {
	now := time.Now().UTC()
	return &mockDashboardStore{
		getTotalsFn: func(_ context.Context, _ port.DashboardStatsParams) (*port.DashboardTotals, error) {
			return &port.DashboardTotals{
				Sent: 100, Delivered: 90, Bounced: 5, Complained: 2, Failed: 3,
			}, nil
		},
		getTimeSeriesFn: func(_ context.Context, _ port.DashboardStatsParams) ([]port.DashboardTimePoint, error) {
			return []port.DashboardTimePoint{
				{Date: now.Truncate(24 * time.Hour), Sent: 50, Delivered: 45, Bounced: 3, Failed: 2},
			}, nil
		},
		getRecentEmailsFn: func(_ context.Context, _ port.DashboardStatsParams, _ int) ([]port.DashboardRecentEmail, error) {
			return []port.DashboardRecentEmail{
				{
					ID: uuid.Must(uuid.NewV7()), TrackingID: "trk-1",
					RecipientEmail: "user@test.com", TemplateTypeSlug: "welcome",
					Status: domain.StatusDelivered, CreatedAt: now,
				},
			}, nil
		},
	}
}

func defaultMockAuditStore() *mockAuditLogStore {
	now := time.Now().UTC()
	return &mockAuditLogStore{
		queryFn: func(_ context.Context, _ port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
			return &port.PageResult[domain.AuditLog]{
				Items: []*domain.AuditLog{
					{
						ID: uuid.Must(uuid.NewV7()), ActorID: uuid.Must(uuid.NewV7()),
						ActorEmail: "admin@acme.com", Action: domain.AuditCreate,
						EntityType: "template", EntityID: uuid.Must(uuid.NewV7()),
						ScopeType: domain.ScopeGlobal, CreatedAt: now,
					},
				},
			}, nil
		},
	}
}

// --- Tests ---

func TestDashboardHandler_Stats_WorkspaceSuccess(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var capturedParams port.DashboardStatsParams
	ds := defaultMockDashboardStore()
	ds.getTotalsFn = func(_ context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error) {
		capturedParams = p
		return &port.DashboardTotals{Sent: 100, Delivered: 90, Bounced: 5, Complained: 2, Failed: 3}, nil
	}

	e := setupDashboardTest(ds, defaultMockAuditStore(), ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/dashboard-stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify workspace scoping.
	if capturedParams.WorkspaceID == nil || *capturedParams.WorkspaceID != ws.ID {
		t.Fatal("expected workspace_id filter to be set")
	}

	var resp response.DashboardStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Totals.Sent != 100 {
		t.Errorf("expected Sent=100, got %d", resp.Totals.Sent)
	}
	if resp.Totals.Delivered != 90 {
		t.Errorf("expected Delivered=90, got %d", resp.Totals.Delivered)
	}
	if len(resp.TimeSeries) != 1 {
		t.Errorf("expected 1 time series point, got %d", len(resp.TimeSeries))
	}
	if len(resp.RecentEmails) != 1 {
		t.Errorf("expected 1 recent email, got %d", len(resp.RecentEmails))
	}
	if len(resp.RecentActivity) != 1 {
		t.Errorf("expected 1 activity item, got %d", len(resp.RecentActivity))
	}
}

func TestDashboardHandler_StatsGlobal_Success(t *testing.T) {
	var capturedParams port.DashboardStatsParams
	ds := defaultMockDashboardStore()
	ds.getTotalsFn = func(_ context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error) {
		capturedParams = p
		return &port.DashboardTotals{Sent: 200, Delivered: 180}, nil
	}

	e := setupDashboardTest(ds, defaultMockAuditStore(), &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/dashboard-stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Global scope: no workspace or tenant filter.
	if capturedParams.WorkspaceID != nil {
		t.Fatal("expected nil workspace_id for global scope")
	}
	if capturedParams.TenantID != nil {
		t.Fatal("expected nil tenant_id for global scope")
	}
}

func TestDashboardHandler_StatsTenant_Success(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()

	var capturedParams port.DashboardStatsParams
	ds := defaultMockDashboardStore()
	ds.getTotalsFn = func(_ context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error) {
		capturedParams = p
		return &port.DashboardTotals{Sent: 150, Delivered: 140}, nil
	}

	e := setupDashboardTest(ds, defaultMockAuditStore(), ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/dashboard-stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if capturedParams.TenantID == nil || *capturedParams.TenantID != tenant.ID {
		t.Fatal("expected tenant_id filter to be set")
	}
	if capturedParams.WorkspaceID != nil {
		t.Fatal("expected nil workspace_id for tenant scope")
	}
}

func TestDashboardHandler_RateComputation(t *testing.T) {
	ds := &mockDashboardStore{
		getTotalsFn: func(_ context.Context, _ port.DashboardStatsParams) (*port.DashboardTotals, error) {
			return &port.DashboardTotals{Sent: 200, Delivered: 180, Bounced: 10, Complained: 4, Failed: 6}, nil
		},
	}

	e := setupDashboardTest(ds, defaultMockAuditStore(), &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/dashboard-stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.DashboardStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// delivery_rate = 180 / 200 = 0.9
	if resp.Rates.DeliveryRate != 0.9 {
		t.Errorf("expected delivery_rate=0.9, got %f", resp.Rates.DeliveryRate)
	}
	// bounce_rate = 10 / 200 = 0.05
	if resp.Rates.BounceRate != 0.05 {
		t.Errorf("expected bounce_rate=0.05, got %f", resp.Rates.BounceRate)
	}
	// complaint_rate = 4 / 200 = 0.02
	if resp.Rates.ComplaintRate != 0.02 {
		t.Errorf("expected complaint_rate=0.02, got %f", resp.Rates.ComplaintRate)
	}
}

func TestDashboardHandler_RateComputation_ZeroDivision(t *testing.T) {
	ds := &mockDashboardStore{
		getTotalsFn: func(_ context.Context, _ port.DashboardStatsParams) (*port.DashboardTotals, error) {
			return &port.DashboardTotals{Sent: 0, Delivered: 0, Bounced: 0, Complained: 0, Failed: 0}, nil
		},
	}

	e := setupDashboardTest(ds, defaultMockAuditStore(), &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/dashboard-stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.DashboardStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Rates.DeliveryRate != 0 {
		t.Errorf("expected delivery_rate=0, got %f", resp.Rates.DeliveryRate)
	}
	if resp.Rates.BounceRate != 0 {
		t.Errorf("expected bounce_rate=0, got %f", resp.Rates.BounceRate)
	}
	if resp.Rates.ComplaintRate != 0 {
		t.Errorf("expected complaint_rate=0, got %f", resp.Rates.ComplaintRate)
	}
}

func TestDashboardHandler_InvalidRangeDefaultsTo7d(t *testing.T) {
	var capturedParams port.DashboardStatsParams
	ds := defaultMockDashboardStore()
	ds.getTotalsFn = func(_ context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error) {
		capturedParams = p
		return &port.DashboardTotals{}, nil
	}

	e := setupDashboardTest(ds, defaultMockAuditStore(), &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/dashboard-stats?range=invalid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Default range is 7 days.
	diff := capturedParams.Until.Sub(capturedParams.Since)
	expectedDiff := 7 * 24 * time.Hour
	if diff < expectedDiff-time.Minute || diff > expectedDiff+time.Minute {
		t.Errorf("expected ~7d range, got %v", diff)
	}
}

func TestDashboardHandler_Range30d(t *testing.T) {
	var capturedParams port.DashboardStatsParams
	ds := defaultMockDashboardStore()
	ds.getTotalsFn = func(_ context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error) {
		capturedParams = p
		return &port.DashboardTotals{}, nil
	}

	e := setupDashboardTest(ds, defaultMockAuditStore(), &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/dashboard-stats?range=30d", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	diff := capturedParams.Until.Sub(capturedParams.Since)
	// 30 days ~ 720 hours. Allow +/- 1 minute for test execution time.
	expectedDiff := 30 * 24 * time.Hour
	if diff < expectedDiff-time.Minute || diff > expectedDiff+time.Minute {
		t.Errorf("expected ~30d range, got %v", diff)
	}
}

func TestDashboardHandler_Stats_InvalidTenant(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}

	e := setupDashboardTest(defaultMockDashboardStore(), defaultMockAuditStore(), ts, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/nonexistent/workspaces/default/dashboard-stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

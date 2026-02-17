package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
	"golang.org/x/sync/errgroup"
)

// DashboardHandler handles dashboard statistics endpoints (OIDC management auth).
type DashboardHandler struct {
	dashStore  port.DashboardStore
	auditStore port.AuditLogStore
	tsStore    port.TenantStore
	wsStore    port.WorkspaceStore
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(ds port.DashboardStore, als port.AuditLogStore, ts port.TenantStore, ws port.WorkspaceStore) *DashboardHandler {
	return &DashboardHandler{dashStore: ds, auditStore: als, tsStore: ts, wsStore: ws}
}

// Stats handles GET /tenants/:tenant_code/workspaces/:workspace_code/dashboard-stats.
func (h *DashboardHandler) Stats(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	params := parseDashboardParams(c)
	params.WorkspaceID = &ws.ID

	auditFilter := port.AuditFilter{
		WorkspaceID: &ws.ID,
		TenantID:    &ws.TenantID,
	}

	return h.fetchAndRespond(c, params, auditFilter)
}

// StatsTenant handles GET /tenants/:tenant_code/dashboard-stats.
func (h *DashboardHandler) StatsTenant(c *echo.Context) error {
	tenant, err := resolveTenant(c, h.tsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	params := parseDashboardParams(c)
	params.TenantID = &tenant.ID

	auditFilter := port.AuditFilter{
		TenantID: &tenant.ID,
	}

	return h.fetchAndRespond(c, params, auditFilter)
}

// StatsGlobal handles GET /global/dashboard-stats.
func (h *DashboardHandler) StatsGlobal(c *echo.Context) error {
	params := parseDashboardParams(c)
	auditFilter := port.AuditFilter{}

	return h.fetchAndRespond(c, params, auditFilter)
}

// fetchAndRespond runs all 4 dashboard queries in parallel and returns the response.
func (h *DashboardHandler) fetchAndRespond(c *echo.Context, params port.DashboardStatsParams, auditFilter port.AuditFilter) error {
	ctx := c.Request().Context()

	var (
		totals       *port.DashboardTotals
		series       []port.DashboardTimePoint
		recentEmails []port.DashboardRecentEmail
		auditLogs    []*domain.AuditLog
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		totals, err = h.dashStore.GetTotals(gCtx, params)
		return err
	})

	g.Go(func() error {
		var err error
		series, err = h.dashStore.GetTimeSeries(gCtx, params)
		return err
	})

	g.Go(func() error {
		var err error
		recentEmails, err = h.dashStore.GetRecentEmails(gCtx, params, 5)
		return err
	})

	g.Go(func() error {
		page, err := h.auditStore.Query(gCtx, auditFilter, port.ListOptions{Limit: 10})
		if err != nil {
			return err
		}
		auditLogs = page.Items
		return nil
	})

	if err := g.Wait(); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewDashboardStatsResponse(totals, series, recentEmails, auditLogs))
}

// parseDashboardParams extracts the time range from the ?range query parameter.
// Supported values: "7d" (default), "30d".
func parseDashboardParams(c *echo.Context) port.DashboardStatsParams {
	now := time.Now().UTC()

	days := 7
	if r := c.QueryParam("range"); r == "30d" {
		days = 30
	}

	return port.DashboardStatsParams{
		Since: now.AddDate(0, 0, -days),
		Until: now,
	}
}

package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/http/pagination"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

// AuditHandler handles audit log query endpoints (OIDC management auth).
type AuditHandler struct {
	store   port.AuditLogStore
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(als port.AuditLogStore, ts port.TenantStore, ws port.WorkspaceStore) *AuditHandler {
	return &AuditHandler{store: als, tsStore: ts, wsStore: ws}
}

// Query handles GET /tenants/:tenant_code/workspaces/:workspace_code/audit-log.
func (h *AuditHandler) Query(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	filter := parseAuditFilter(c)
	filter.WorkspaceID = &ws.ID

	// Set tenant from the workspace's tenant.
	filter.TenantID = &ws.TenantID

	opts := pagination.ParseListOptions(c)

	page, err := h.store.Query(c.Request().Context(), filter, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAuditLogListResponse(page))
}

// QueryGlobal handles GET /global/audit-log.
func (h *AuditHandler) QueryGlobal(c *echo.Context) error {
	filter := parseAuditFilter(c)
	opts := pagination.ParseListOptions(c)

	page, err := h.store.Query(c.Request().Context(), filter, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAuditLogListResponse(page))
}

// parseAuditFilter extracts audit log filter query parameters.
func parseAuditFilter(c *echo.Context) port.AuditFilter {
	var f port.AuditFilter

	if s := c.QueryParam("actor_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			f.ActorID = &id
		}
	}
	if s := c.QueryParam("action"); s != "" {
		f.Action = &s
	}
	if s := c.QueryParam("entity_type"); s != "" {
		f.EntityType = &s
	}
	if s := c.QueryParam("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.Since = &t
		}
	}
	if s := c.QueryParam("until"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.Until = &t
		}
	}

	return f
}

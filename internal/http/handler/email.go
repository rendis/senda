package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// EmailHandler handles email query endpoints (OIDC management auth).
type EmailHandler struct {
	store   port.EmailStore
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewEmailHandler creates a new EmailHandler.
func NewEmailHandler(es port.EmailStore, ts port.TenantStore, ws port.WorkspaceStore) *EmailHandler {
	return &EmailHandler{store: es, tsStore: ts, wsStore: ws}
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/emails.
func (h *EmailHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	cursor := c.QueryParam("cursor")
	limit := parseLimit(c)
	filters := parseEmailFilters(c)

	emails, nextCursor, err := h.store.QueryByWorkspace(c.Request().Context(), ws.ID, filters, cursor, limit)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewEmailListResponse(emails, nextCursor))
}

// GetByTrackingID handles GET /tenants/:tenant_code/workspaces/:workspace_code/emails/:tracking_id.
func (h *EmailHandler) GetByTrackingID(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	trackingID := c.Param("tracking_id")
	if trackingID == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "tracking_id is required")
	}

	ctx := c.Request().Context()
	email, err := h.store.GetByTrackingID(ctx, trackingID)
	if err != nil {
		return mapStoreError(c, err)
	}

	// Verify workspace ownership.
	if email.WorkspaceID != ws.ID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	events, err := h.store.GetEvents(ctx, email.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewEmailDetailResponse(email, events))
}

// GetEvents handles GET /tenants/:tenant_code/workspaces/:workspace_code/emails/:tracking_id/events.
func (h *EmailHandler) GetEvents(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	trackingID := c.Param("tracking_id")
	if trackingID == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "tracking_id is required")
	}

	ctx := c.Request().Context()
	email, err := h.store.GetByTrackingID(ctx, trackingID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if email.WorkspaceID != ws.ID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	events, err := h.store.GetEvents(ctx, email.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewEmailEventListResponse(events))
}

// parseEmailFilters extracts email filter query parameters.
func parseEmailFilters(c *echo.Context) port.EmailFilters {
	var f port.EmailFilters

	if s := c.QueryParam("external_id"); s != "" {
		f.ExternalID = &s
	}
	if s := c.QueryParam("recipient"); s != "" {
		f.Recipient = &s
	}
	if s := c.QueryParam("status"); s != "" {
		status := domain.EmailStatus(s)
		f.Status = &status
	}
	if s := c.QueryParam("template_type"); s != "" {
		f.TemplateTypeSlug = &s
	}
	if s := c.QueryParam("adapter_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			f.AdapterID = &id
		}
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

// parseLimit extracts the limit query parameter with defaults.
func parseLimit(c *echo.Context) int {
	limit := 25
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

package handler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// resolveWorkspace looks up a workspace by :tenant_code and :workspace_code path params.
func resolveWorkspace(c *echo.Context, ts port.TenantStore, ws port.WorkspaceStore) (*domain.Workspace, error) {
	tenantCode := c.Param("tenant_code")
	wsCode := c.Param("workspace_code")
	ctx := c.Request().Context()

	tenant, err := ts.GetByCode(ctx, tenantCode)
	if err != nil {
		return nil, err
	}

	workspace, err := ws.GetByTenantAndCode(ctx, tenant.ID, wsCode)
	if err != nil {
		return nil, err
	}

	return workspace, nil
}

// resolveTenant looks up a tenant by :tenant_code path param.
func resolveTenant(c *echo.Context, ts port.TenantStore) (*domain.Tenant, error) {
	tenantCode := c.Param("tenant_code")
	return ts.GetByCode(c.Request().Context(), tenantCode)
}

// parseOptionalUUID converts an optional string pointer to *uuid.UUID.
// Returns (nil, nil) when s is nil.
func parseOptionalUUID(s *string) (*uuid.UUID, error) {
	if s == nil {
		return nil, nil
	}
	parsed, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

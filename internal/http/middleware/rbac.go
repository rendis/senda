package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// RequireRole returns middleware that enforces RBAC based on the minimum role required.
// API key authentication bypasses RBAC (API keys are workspace-scoped, data-plane only).
// OIDC authentication checks the member's roles against the required minimum role
// within the resolved tenant/workspace scope.
func RequireRole(minRole domain.Role, tenantStore port.TenantStore, wsStore port.WorkspaceStore) echo.MiddlewareFunc { //nolint:gocognit // auth type, tenant, and workspace scope resolution branch by design
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			authType, _ := c.Get(ContextKeyAuthType).(string)

			// API key auth bypasses RBAC.
			if authType == "apikey" {
				return next(c)
			}

			// Must be OIDC-authenticated.
			if authType != "oidc" {
				return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "authentication required")
			}

			roles, _ := c.Get(ContextKeyRoles).([]*domain.MemberRole)
			if roles == nil {
				return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "no roles assigned")
			}

			ctx := c.Request().Context()

			// Resolve tenant ID from tenant_code if present.
			var tenantID *uuid.UUID
			if tc, _ := c.Get(ContextKeyTenantCode).(string); tc != "" {
				tenant, err := tenantStore.GetByCode(ctx, tc)
				if err != nil {
					return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "invalid tenant")
				}
				tenantID = &tenant.ID
			}

			// Resolve workspace ID from workspace_code if present.
			var workspaceID *uuid.UUID
			if wc, _ := c.Get(ContextKeyWorkspaceCode).(string); wc != "" && tenantID != nil {
				environment, _ := c.Get(ContextKeyEnvironment).(domain.Environment)
				if !environment.Valid() {
					environment = domain.EnvironmentProd
				}
				ws, err := wsStore.GetByTenantAndCode(ctx, *tenantID, wc, environment)
				if err != nil {
					return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "invalid workspace")
				}
				workspaceID = &ws.ID
			}

			if hasPermission(roles, minRole, tenantID, workspaceID) {
				return next(c)
			}

			return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
		}
	}
}

func hasPermission(roles []*domain.MemberRole, minRole domain.Role, tenantID, workspaceID *uuid.UUID) bool {
	for _, role := range roles {
		// Superadmin with global scope bypasses all checks.
		// Defense-in-depth: only ScopeGlobal superadmin gets full access.
		if role.Role == domain.RoleSuperadmin && role.ScopeType == domain.ScopeGlobal {
			return true
		}

		// Tenant-scoped role: matches tenant, and also covers workspaces within that tenant.
		if role.ScopeType == domain.ScopeTenant && tenantID != nil && role.TenantID != nil && *role.TenantID == *tenantID {
			if role.Role.Level() >= minRole.Level() {
				return true
			}
		}

		// Workspace-scoped role: must match the exact workspace.
		if role.ScopeType == domain.ScopeWorkspace && workspaceID != nil && role.WorkspaceID != nil && *role.WorkspaceID == *workspaceID {
			if role.Role.Level() >= minRole.Level() {
				return true
			}
		}
	}
	return false
}

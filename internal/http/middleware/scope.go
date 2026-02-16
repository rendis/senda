package middleware

import (
	"github.com/labstack/echo/v5"
)

const (
	// ContextKeyTenantCode is the echo context key for the tenant code.
	ContextKeyTenantCode = "tenant_code"

	// ContextKeyWorkspaceCode is the echo context key for the workspace code.
	ContextKeyWorkspaceCode = "workspace_code"
)

// Scope returns middleware that extracts :tenant_code and :workspace_code
// from URL path parameters and stores them in the echo context.
// Only sets values when the corresponding path params exist.
func Scope() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if tc := c.Param("tenant_code"); tc != "" {
				c.Set(ContextKeyTenantCode, tc)
			}
			if wc := c.Param("workspace_code"); wc != "" {
				c.Set(ContextKeyWorkspaceCode, wc)
			}

			return next(c)
		}
	}
}

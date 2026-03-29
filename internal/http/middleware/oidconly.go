package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/response"
)

// OIDCOnly returns middleware that rejects non-OIDC authentication.
// Management API endpoints require OIDC authentication; API keys are
// restricted to data-plane operations.
func OIDCOnly() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			authType, _ := c.Get(ContextKeyAuthType).(string)
			if authType != "oidc" {
				return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "management API requires OIDC authentication")
			}
			return next(c)
		}
	}
}

package middleware

import (
	"log/slog"
	"net/http"
	"runtime"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/response"
)

// Recovery returns middleware that recovers from panics in downstream handlers.
// On panic it logs the error with a stack trace and returns a 500 JSON response.
func Recovery(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (retErr error) {
			defer func() {
				if r := recover(); r != nil {
					buf := make([]byte, 4096)
					n := runtime.Stack(buf, false)
					stack := string(buf[:n])

					logger.Error("panic recovered",
						slog.Any("error", r),
						slog.String("stack", stack),
					)

					retErr = response.WriteError(c, http.StatusInternalServerError,
						"INTERNAL_ERROR",
						"internal server error",
					)
				}
			}()

			return next(c)
		}
	}
}

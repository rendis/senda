package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"
)

// Logger returns middleware that logs each request using slog in JSON format.
// It records method, path, status, duration_ms, request_id, and remote_ip.
func Logger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()

			err := next(c)

			reqID, _ := c.Get(ContextKeyRequestID).(string)

			status := 0
			if resp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil {
				status = resp.Status
			}

			logger.Info("request",
				slog.String("method", c.Request().Method),
				slog.String("path", c.Request().URL.Path),
				slog.Int("status", status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("request_id", reqID),
				slog.String("remote_ip", c.RealIP()),
			)

			return err
		}
	}
}

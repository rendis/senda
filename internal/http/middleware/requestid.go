package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	// HeaderXRequestID is the header key for the request ID.
	HeaderXRequestID = "X-Request-ID"

	// ContextKeyRequestID is the echo context key for the request ID.
	ContextKeyRequestID = "request_id"
)

// RequestID returns middleware that generates or propagates a request ID.
// If the incoming request has an X-Request-ID header, that value is used.
// Otherwise a new UUID v4 is generated.
// The ID is stored in the echo context and set on the response header.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			reqID := c.Request().Header.Get(HeaderXRequestID)
			if reqID == "" {
				reqID = uuid.New().String()
			}

			c.Set(ContextKeyRequestID, reqID)
			c.Response().Header().Set(HeaderXRequestID, reqID)

			return next(c)
		}
	}
}

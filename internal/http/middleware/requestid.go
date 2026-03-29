package middleware

import (
	"regexp"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	// HeaderXRequestID is the header key for the request ID.
	HeaderXRequestID = "X-Request-ID"

	// ContextKeyRequestID is the echo context key for the request ID.
	ContextKeyRequestID = "request_id"

	// maxRequestIDLength is the maximum allowed length for an incoming request ID.
	maxRequestIDLength = 128
)

// validRequestID matches only alphanumeric characters, hyphens, underscores, and periods.
var validRequestID = regexp.MustCompile(`^[a-zA-Z0-9\-_.]+$`)

// RequestID returns middleware that generates or propagates a request ID.
// If the incoming request has a valid X-Request-ID header, that value is used.
// The ID is considered valid when it is non-empty, at most 128 characters,
// and contains only alphanumeric characters, hyphens, underscores, or periods.
// If the header is missing or invalid, a new UUID v4 is generated.
// The ID is stored in the echo context and set on the response header.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			reqID := c.Request().Header.Get(HeaderXRequestID)
			if !isValidRequestID(reqID) {
				reqID = uuid.New().String()
			}

			c.Set(ContextKeyRequestID, reqID)
			c.Response().Header().Set(HeaderXRequestID, reqID)

			return next(c)
		}
	}
}

// isValidRequestID checks that the request ID is non-empty, within the max
// length, and contains only allowed characters (alphanumeric, hyphen,
// underscore, period).
func isValidRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}
	return validRequestID.MatchString(id)
}

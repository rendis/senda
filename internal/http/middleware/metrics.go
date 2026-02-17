package middleware

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/metrics"
)

// uuidRegex matches UUID v1-v7 patterns in URL path segments.
var uuidRegex = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// Metrics returns middleware that records HTTP request duration and count.
// It should be placed after RequestID and before Logger in the middleware chain.
func Metrics() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()

			err := next(c)

			status := 0
			if resp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil {
				status = resp.Status
			}

			path := metricsPath(c)
			method := c.Request().Method
			statusStr := strconv.Itoa(status)

			metrics.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(time.Since(start).Seconds())

			return err
		}
	}
}

// metricsPath returns the route pattern if available, otherwise normalizes the raw path.
func metricsPath(c *echo.Context) string {
	if ri := c.RouteInfo(); ri.Path != "" {
		return ri.Path
	}
	return NormalizePath(c.Request().URL.Path)
}

// NormalizePath replaces UUID segments in a URL path with ":id" to prevent
// high-cardinality metric labels. Exported for testing.
func NormalizePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if uuidRegex.MatchString(seg) {
			segments[i] = ":id"
		}
	}
	return strings.Join(segments, "/")
}

package handler

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
)

// Pinger is satisfied by *pgxpool.Pool and allows testing without a real DB.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves health check endpoints.
type HealthHandler struct {
	pinger Pinger
}

// NewHealthHandler creates a HealthHandler. If pinger is nil, the DB check is skipped.
func NewHealthHandler(pinger Pinger) *HealthHandler {
	return &HealthHandler{pinger: pinger}
}

// healthResponse is the JSON structure returned by the health endpoint.
type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Health handles GET /healthz. It checks DB connectivity and returns status.
func (h *HealthHandler) Health(c *echo.Context) error {
	if h.pinger == nil {
		return c.JSON(http.StatusOK, healthResponse{Status: "healthy"})
	}

	if err := h.pinger.Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, healthResponse{
			Status: "unhealthy",
			Checks: map[string]string{"database": err.Error()},
		})
	}

	return c.JSON(http.StatusOK, healthResponse{
		Status: "healthy",
		Checks: map[string]string{"database": "ok"},
	})
}

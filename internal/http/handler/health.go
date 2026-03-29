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

// RiverChecker exposes a method to verify the River job queue is operational.
// Satisfied by the river adapter Client (non-nil check + pool ping).
type RiverChecker interface {
	Healthy(ctx context.Context) error
}

// HealthHandler serves health check endpoints.
type HealthHandler struct {
	pinger       Pinger
	riverChecker RiverChecker
}

// NewHealthHandler creates a HealthHandler. If pinger is nil, the DB check is skipped.
func NewHealthHandler(pinger Pinger, opts ...HealthOption) *HealthHandler {
	h := &HealthHandler{pinger: pinger}
	for _, o := range opts {
		o(h)
	}
	return h
}

// HealthOption configures optional HealthHandler dependencies.
type HealthOption func(*HealthHandler)

// WithRiverChecker sets the River health checker.
func WithRiverChecker(rc RiverChecker) HealthOption {
	return func(h *HealthHandler) {
		h.riverChecker = rc
	}
}

// healthResponse is the JSON structure returned by the health endpoint.
type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Health handles GET /healthz. It checks DB connectivity and River queue status.
func (h *HealthHandler) Health(c *echo.Context) error {
	ctx := c.Request().Context()
	checks := make(map[string]string)
	healthy := true

	if h.pinger != nil {
		if err := h.pinger.Ping(ctx); err != nil {
			checks["database"] = err.Error()
			healthy = false
		} else {
			checks["database"] = "ok"
		}
	}

	if h.riverChecker != nil {
		if err := h.riverChecker.Healthy(ctx); err != nil {
			checks["river"] = err.Error()
			healthy = false
		} else {
			checks["river"] = "ok"
		}
	}

	status := "healthy"
	code := http.StatusOK
	if !healthy {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}

	return c.JSON(code, healthResponse{
		Status: status,
		Checks: checks,
	})
}

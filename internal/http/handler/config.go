package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/http/request"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

// ConfigHandler handles operations on the global configuration.
type ConfigHandler struct {
	store port.GlobalConfigStore
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(cs port.GlobalConfigStore) *ConfigHandler {
	return &ConfigHandler{store: cs}
}

// Get handles GET /api/v1/manage/config.
func (h *ConfigHandler) Get(c *echo.Context) error {
	cfg, err := h.store.Get(c.Request().Context())
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewConfigResponse(cfg))
}

// Update handles PUT /api/v1/manage/config (partial update).
func (h *ConfigHandler) Update(c *echo.Context) error {
	ctx := c.Request().Context()
	cfg, err := h.store.Get(ctx)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.UpdateConfigRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.DefaultRetryCount != nil {
		cfg.DefaultRetryCount = *req.DefaultRetryCount
	}
	if req.RetryBackoffBaseSeconds != nil {
		cfg.RetryBackoffBaseSeconds = *req.RetryBackoffBaseSeconds
	}
	if req.LogRetentionDays != nil {
		cfg.LogRetentionDays = *req.LogRetentionDays
	}
	if req.BounceAlertThresholdPercent != nil {
		cfg.BounceAlertThresholdPercent = *req.BounceAlertThresholdPercent
	}
	if req.ComplaintAlertThresholdPercent != nil {
		cfg.ComplaintAlertThresholdPercent = *req.ComplaintAlertThresholdPercent
	}
	if req.DomainRecheckIntervalHours != nil {
		cfg.DomainRecheckIntervalHours = *req.DomainRecheckIntervalHours
	}
	if req.OnboardingCompleted != nil {
		cfg.OnboardingCompleted = *req.OnboardingCompleted
	}

	cfg.UpdatedAt = time.Now().UTC()
	if err := h.store.Upsert(ctx, cfg); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewConfigResponse(cfg))
}

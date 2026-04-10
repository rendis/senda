package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// OIDCInfo holds read-only OIDC configuration exposed in the config response.
type OIDCInfo struct {
	DiscoveryURL    string
	ClientID        string
	ClientSecretSet bool
}

// ConfigHandler handles operations on the global configuration.
type ConfigHandler struct {
	store port.GlobalConfigStore
	oidc  OIDCInfo
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(cs port.GlobalConfigStore, oidc OIDCInfo) *ConfigHandler {
	return &ConfigHandler{store: cs, oidc: oidc}
}

// Get handles GET /api/v1/manage/config.
func (h *ConfigHandler) Get(c *echo.Context) error {
	cfg, err := h.store.Get(c.Request().Context())
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewConfigResponse(cfg, h.oidc.DiscoveryURL, h.oidc.ClientID, h.oidc.ClientSecretSet))
}

// Update handles PUT /api/v1/manage/config (partial update, nested format).
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

	if req.EmailDefaults != nil {
		if req.EmailDefaults.MaxRetries != nil {
			cfg.DefaultRetryCount = *req.EmailDefaults.MaxRetries
		}
		if req.EmailDefaults.BackoffBaseSeconds != nil {
			cfg.RetryBackoffBaseSeconds = *req.EmailDefaults.BackoffBaseSeconds
		}
		if req.EmailDefaults.LogRetentionDays != nil {
			cfg.LogRetentionDays = *req.EmailDefaults.LogRetentionDays
		}
	}

	if req.Alerts != nil {
		if req.Alerts.BounceThresholdPercent != nil {
			cfg.BounceAlertThresholdPercent = *req.Alerts.BounceThresholdPercent
		}
		if req.Alerts.ComplaintThresholdPercent != nil {
			cfg.ComplaintAlertThresholdPercent = *req.Alerts.ComplaintThresholdPercent
		}
	}

	if req.Domain != nil {
		if req.Domain.RecheckIntervalHours != nil {
			cfg.DomainRecheckIntervalHours = *req.Domain.RecheckIntervalHours
		}
	}

	if req.ExternalIntegrations != nil {
		cfg.ExternalIntegrations = domain.NormalizeExternalIntegrationProfiles(req.ExternalIntegrations.ToDomain())
	}

	if err := cfg.Validate(); err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	}

	cfg.UpdatedAt = time.Now().UTC()
	if err := h.store.Upsert(ctx, cfg); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewConfigResponse(cfg, h.oidc.DiscoveryURL, h.oidc.ClientID, h.oidc.ClientSecretSet))
}

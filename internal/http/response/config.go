package response

import (
	"github.com/senda-app/senda/internal/domain"
)

// ConfigResponse is the JSON response for the global configuration.
type ConfigResponse struct {
	DefaultRetryCount             int     `json:"default_retry_count"`
	RetryBackoffBaseSeconds       int     `json:"retry_backoff_base_seconds"`
	LogRetentionDays              int     `json:"log_retention_days"`
	BounceAlertThresholdPercent   float64 `json:"bounce_alert_threshold_percent"`
	ComplaintAlertThresholdPercent float64 `json:"complaint_alert_threshold_percent"`
	DomainRecheckIntervalHours    int     `json:"domain_recheck_interval_hours"`
	OnboardingCompleted           bool    `json:"onboarding_completed"`
	UpdatedAt                     string  `json:"updated_at"`
}

// NewConfigResponse maps a domain GlobalConfig to a ConfigResponse.
func NewConfigResponse(cfg *domain.GlobalConfig) ConfigResponse {
	return ConfigResponse{
		DefaultRetryCount:             cfg.DefaultRetryCount,
		RetryBackoffBaseSeconds:       cfg.RetryBackoffBaseSeconds,
		LogRetentionDays:              cfg.LogRetentionDays,
		BounceAlertThresholdPercent:   cfg.BounceAlertThresholdPercent,
		ComplaintAlertThresholdPercent: cfg.ComplaintAlertThresholdPercent,
		DomainRecheckIntervalHours:    cfg.DomainRecheckIntervalHours,
		OnboardingCompleted:           cfg.OnboardingCompleted,
		UpdatedAt:                     cfg.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

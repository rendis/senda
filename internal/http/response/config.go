package response

import (
	"github.com/rendis/senda/internal/domain"
)

// OIDCConfigResponse is the read-only OIDC section of the config response.
type OIDCConfigResponse struct {
	DiscoveryURL    string `json:"discovery_url"`
	ClientID        string `json:"client_id"`
	ClientSecretSet bool   `json:"client_secret_set"`
}

// EmailDefaultsConfigResponse is the email defaults section.
type EmailDefaultsConfigResponse struct {
	MaxRetries         int `json:"max_retries"`
	BackoffBaseSeconds int `json:"backoff_base_seconds"`
	LogRetentionDays   int `json:"log_retention_days"`
}

// AlertsConfigResponse is the alerts section.
type AlertsConfigResponse struct {
	BounceThresholdPercent    float64 `json:"bounce_threshold_percent"`
	ComplaintThresholdPercent float64 `json:"complaint_threshold_percent"`
}

// DomainConfigResponse is the domain section.
type DomainConfigResponse struct {
	RecheckIntervalHours int `json:"recheck_interval_hours"`
}

// ConfigResponse is the nested JSON response for the global configuration.
// Matches the frontend SystemSettings type.
type ConfigResponse struct {
	OIDC          OIDCConfigResponse          `json:"oidc"`
	EmailDefaults EmailDefaultsConfigResponse `json:"email_defaults"`
	Alerts        AlertsConfigResponse        `json:"alerts"`
	Domain        DomainConfigResponse        `json:"domain"`
}

// NewConfigResponse maps a domain GlobalConfig + OIDC info to a ConfigResponse.
func NewConfigResponse(cfg *domain.GlobalConfig, oidcDiscoveryURL, oidcClientID string, oidcClientSecretSet bool) ConfigResponse {
	return ConfigResponse{
		OIDC: OIDCConfigResponse{
			DiscoveryURL:    oidcDiscoveryURL,
			ClientID:        oidcClientID,
			ClientSecretSet: oidcClientSecretSet,
		},
		EmailDefaults: EmailDefaultsConfigResponse{
			MaxRetries:         cfg.DefaultRetryCount,
			BackoffBaseSeconds: cfg.RetryBackoffBaseSeconds,
			LogRetentionDays:   cfg.LogRetentionDays,
		},
		Alerts: AlertsConfigResponse{
			BounceThresholdPercent:    cfg.BounceAlertThresholdPercent,
			ComplaintThresholdPercent: cfg.ComplaintAlertThresholdPercent,
		},
		Domain: DomainConfigResponse{
			RecheckIntervalHours: cfg.DomainRecheckIntervalHours,
		},
	}
}

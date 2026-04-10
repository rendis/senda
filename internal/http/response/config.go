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

// ExternalIntegrationCapabilitiesResponse mirrors the domain capabilities.
type ExternalIntegrationCapabilitiesResponse struct {
	ListTemplates   bool `json:"list_templates"`
	ViewVersions    bool `json:"view_versions"`
	EditVersions    bool `json:"edit_versions"`
	PublishVersions bool `json:"publish_versions"`
	TestSend        bool `json:"test_send"`
	BuilderAccess   bool `json:"builder_access"`
	MetadataAccess  bool `json:"metadata_access"`
	LocaleAccess    bool `json:"locale_access"`
}

// ExternalIntegrationProfileResponse exposes one external integration profile
// in the global settings payload.
type ExternalIntegrationProfileResponse struct {
	Slug            string                                  `json:"slug"`
	Name            string                                  `json:"name"`
	Description     string                                  `json:"description"`
	Enabled         bool                                    `json:"enabled"`
	AuthMethodName  string                                  `json:"auth_method_name"`
	ResolverName    string                                  `json:"resolver_name"`
	AllowedOrigins  []string                                `json:"allowed_origins"`
	AllowedHeaders  []string                                `json:"allowed_headers"`
	RequiredHeaders []string                                `json:"required_headers"`
	Capabilities    ExternalIntegrationCapabilitiesResponse `json:"capabilities"`
}

// ExternalIntegrationsConfigResponse groups all external integration profiles.
type ExternalIntegrationsConfigResponse struct {
	Profiles []ExternalIntegrationProfileResponse `json:"profiles"`
}

// ExternalIntegrationBootstrapResponse is returned by the external bootstrap
// surface and only exposes the effective framing policy for the embed shell.
type ExternalIntegrationBootstrapResponse struct {
	FrameAncestors []string `json:"frame_ancestors"`
}

// ExternalIntegrationSessionResponse is returned by the authenticated external
// embed state endpoint so the UI can render the effective permissions.
type ExternalIntegrationSessionResponse struct {
	ReadOnly               bool                                    `json:"read_only"`
	EffectiveWorkspaceCode string                                  `json:"effective_workspace_code"`
	Permissions            ExternalIntegrationCapabilitiesResponse `json:"permissions"`
}

// ConfigResponse is the nested JSON response for the global configuration.
// Matches the frontend SystemSettings type.
type ConfigResponse struct {
	OIDC                 OIDCConfigResponse                 `json:"oidc"`
	EmailDefaults        EmailDefaultsConfigResponse        `json:"email_defaults"`
	Alerts               AlertsConfigResponse               `json:"alerts"`
	Domain               DomainConfigResponse               `json:"domain"`
	ExternalIntegrations ExternalIntegrationsConfigResponse `json:"external_integrations"`
}

// NewConfigResponse maps a domain GlobalConfig + OIDC info to a ConfigResponse.
func NewConfigResponse(cfg *domain.GlobalConfig, oidcDiscoveryURL, oidcClientID string, oidcClientSecretSet bool) ConfigResponse {
	profiles := make([]ExternalIntegrationProfileResponse, 0, len(cfg.ExternalIntegrations))
	for _, profile := range cfg.ExternalIntegrations {
		profiles = append(profiles, ExternalIntegrationProfileResponse{
			Slug:            profile.Slug,
			Name:            profile.Name,
			Description:     profile.Description,
			Enabled:         profile.Enabled,
			AuthMethodName:  profile.AuthMethodName,
			ResolverName:    profile.ResolverName,
			AllowedOrigins:  append([]string(nil), profile.AllowedOrigins...),
			AllowedHeaders:  append([]string(nil), profile.AllowedHeaders...),
			RequiredHeaders: append([]string(nil), profile.RequiredHeaders...),
			Capabilities: ExternalIntegrationCapabilitiesResponse{
				ListTemplates:   profile.Capabilities.ListTemplates,
				ViewVersions:    profile.Capabilities.ViewVersions,
				EditVersions:    profile.Capabilities.EditVersions,
				PublishVersions: profile.Capabilities.PublishVersions,
				TestSend:        profile.Capabilities.TestSend,
				BuilderAccess:   profile.Capabilities.BuilderAccess,
				MetadataAccess:  profile.Capabilities.MetadataAccess,
				LocaleAccess:    profile.Capabilities.LocaleAccess,
			},
		})
	}

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
		ExternalIntegrations: ExternalIntegrationsConfigResponse{
			Profiles: profiles,
		},
	}
}

// NewExternalIntegrationBootstrapResponse converts the effective framing
// policy to the minimal bootstrap payload used by the external embed surface.
func NewExternalIntegrationBootstrapResponse(frameAncestors []string) ExternalIntegrationBootstrapResponse {
	return ExternalIntegrationBootstrapResponse{
		FrameAncestors: frameAncestors,
	}
}

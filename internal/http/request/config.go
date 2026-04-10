package request

import "github.com/rendis/senda/internal/domain"

// EmailDefaultsUpdate is the email defaults section of the update request.
type EmailDefaultsUpdate struct {
	MaxRetries         *int `json:"max_retries"`
	BackoffBaseSeconds *int `json:"backoff_base_seconds"`
	LogRetentionDays   *int `json:"log_retention_days"`
}

// AlertsUpdate is the alerts section of the update request.
type AlertsUpdate struct {
	BounceThresholdPercent    *float64 `json:"bounce_threshold_percent"`
	ComplaintThresholdPercent *float64 `json:"complaint_threshold_percent"`
}

// DomainUpdate is the domain section of the update request.
type DomainUpdate struct {
	RecheckIntervalHours *int `json:"recheck_interval_hours"`
}

// ExternalIntegrationCapabilitiesUpdate is the capabilities block for a single profile.
type ExternalIntegrationCapabilitiesUpdate struct {
	ListTemplates   bool `json:"list_templates"`
	ViewVersions    bool `json:"view_versions"`
	EditVersions    bool `json:"edit_versions"`
	PublishVersions bool `json:"publish_versions"`
	TestSend        bool `json:"test_send"`
	BuilderAccess   bool `json:"builder_access"`
	MetadataAccess  bool `json:"metadata_access"`
	LocaleAccess    bool `json:"locale_access"`
}

// ExternalIntegrationProfileUpdate is one external integration profile payload.
type ExternalIntegrationProfileUpdate struct {
	Slug            string                                `json:"slug"`
	Name            string                                `json:"name"`
	Description     string                                `json:"description"`
	Enabled         bool                                  `json:"enabled"`
	AuthMethodName  string                                `json:"auth_method_name"`
	ResolverName    string                                `json:"resolver_name"`
	AllowedOrigins  []string                              `json:"allowed_origins"`
	AllowedHeaders  []string                              `json:"allowed_headers"`
	RequiredHeaders []string                              `json:"required_headers"`
	Capabilities    ExternalIntegrationCapabilitiesUpdate `json:"capabilities"`
}

func (u ExternalIntegrationProfileUpdate) ToDomain() domain.ExternalIntegrationProfile {
	return domain.ExternalIntegrationProfile{
		Slug:            u.Slug,
		Name:            u.Name,
		Description:     u.Description,
		Enabled:         u.Enabled,
		AuthMethodName:  u.AuthMethodName,
		ResolverName:    u.ResolverName,
		AllowedOrigins:  append([]string(nil), u.AllowedOrigins...),
		AllowedHeaders:  append([]string(nil), u.AllowedHeaders...),
		RequiredHeaders: append([]string(nil), u.RequiredHeaders...),
		Capabilities: domain.ExternalIntegrationCapabilities{
			ListTemplates:   u.Capabilities.ListTemplates,
			ViewVersions:    u.Capabilities.ViewVersions,
			EditVersions:    u.Capabilities.EditVersions,
			PublishVersions: u.Capabilities.PublishVersions,
			TestSend:        u.Capabilities.TestSend,
			BuilderAccess:   u.Capabilities.BuilderAccess,
			MetadataAccess:  u.Capabilities.MetadataAccess,
			LocaleAccess:    u.Capabilities.LocaleAccess,
		},
	}
}

// ExternalIntegrationsUpdate replaces the external integration profiles list.
type ExternalIntegrationsUpdate struct {
	Profiles []ExternalIntegrationProfileUpdate `json:"profiles"`
}

func (u *ExternalIntegrationsUpdate) ToDomain() []domain.ExternalIntegrationProfile {
	if u == nil {
		return nil
	}
	profiles := make([]domain.ExternalIntegrationProfile, len(u.Profiles))
	for i, profile := range u.Profiles {
		profiles[i] = profile.ToDomain()
	}
	return profiles
}

// UpdateConfigRequest is the nested request body for PUT /api/v1/manage/config.
// Matches the frontend UpdateSettingsRequest type.
type UpdateConfigRequest struct {
	EmailDefaults        *EmailDefaultsUpdate        `json:"email_defaults"`
	Alerts               *AlertsUpdate               `json:"alerts"`
	Domain               *DomainUpdate               `json:"domain"`
	ExternalIntegrations *ExternalIntegrationsUpdate `json:"external_integrations"`
}

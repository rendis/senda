package request

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

// UpdateConfigRequest is the nested request body for PUT /api/v1/manage/config.
// Matches the frontend UpdateSettingsRequest type.
type UpdateConfigRequest struct {
	EmailDefaults *EmailDefaultsUpdate `json:"email_defaults"`
	Alerts        *AlertsUpdate        `json:"alerts"`
	Domain        *DomainUpdate        `json:"domain"`
}

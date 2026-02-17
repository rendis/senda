package request

// UpdateConfigRequest is the request body for PUT /api/v1/manage/config.
// All fields are pointers to support partial updates.
type UpdateConfigRequest struct {
	DefaultRetryCount             *int     `json:"default_retry_count"`
	RetryBackoffBaseSeconds       *int     `json:"retry_backoff_base_seconds"`
	LogRetentionDays              *int     `json:"log_retention_days"`
	BounceAlertThresholdPercent   *float64 `json:"bounce_alert_threshold_percent"`
	ComplaintAlertThresholdPercent *float64 `json:"complaint_alert_threshold_percent"`
	DomainRecheckIntervalHours    *int     `json:"domain_recheck_interval_hours"`
	OnboardingCompleted           *bool    `json:"onboarding_completed"`
}

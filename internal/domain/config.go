package domain

import "time"

type GlobalConfig struct {
	DefaultRetryCount             int
	RetryBackoffBaseSeconds       int
	LogRetentionDays              int
	BounceAlertThresholdPercent   float64
	ComplaintAlertThresholdPercent float64
	DomainRecheckIntervalHours    int
	OnboardingCompleted           bool
	UpdatedAt                     time.Time
}

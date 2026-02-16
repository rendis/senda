package domain

import (
	"time"

	"github.com/google/uuid"
)

type GlobalConfig struct {
	ID                        uuid.UUID
	DefaultRateLimitPerSecond int
	MaxRecipientsPerRequest   int
	RetentionDays             int
	MaintenanceMode           bool
	UpdatedAt                 time.Time
}

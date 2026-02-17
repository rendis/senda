package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/senda-app/senda/internal/domain"
)

// GlobalConfigRepo implements port.GlobalConfigStore using PostgreSQL.
type GlobalConfigRepo struct {
	pool *pgxpool.Pool
}

// NewGlobalConfigRepo creates a new GlobalConfigRepo.
func NewGlobalConfigRepo(pool *pgxpool.Pool) *GlobalConfigRepo {
	return &GlobalConfigRepo{pool: pool}
}

func (r *GlobalConfigRepo) Get(ctx context.Context) (*domain.GlobalConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT key, value, updated_at FROM global_config`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying global_config: %w", err)
	}
	defer rows.Close()

	cfg := &domain.GlobalConfig{}
	var latestUpdatedAt time.Time

	for rows.Next() {
		var key string
		var value json.RawMessage
		var updatedAt time.Time

		if err := rows.Scan(&key, &value, &updatedAt); err != nil {
			return nil, fmt.Errorf("scanning global_config row: %w", err)
		}

		if updatedAt.After(latestUpdatedAt) {
			latestUpdatedAt = updatedAt
		}

		if err := mapConfigValue(cfg, key, value); err != nil {
			return nil, fmt.Errorf("mapping config key %q: %w", key, err)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating global_config rows: %w", err)
	}

	cfg.UpdatedAt = latestUpdatedAt
	return cfg, nil
}

func (r *GlobalConfigRepo) Upsert(ctx context.Context, cfg *domain.GlobalConfig) error {
	entries := []struct {
		key   string
		value any
	}{
		{"email.default_retry_count", cfg.DefaultRetryCount},
		{"email.retry_backoff_base_seconds", cfg.RetryBackoffBaseSeconds},
		{"email.log_retention_days", cfg.LogRetentionDays},
		{"bounce.alert_threshold_percent", cfg.BounceAlertThresholdPercent},
		{"complaint.alert_threshold_percent", cfg.ComplaintAlertThresholdPercent},
		{"domain.recheck_interval_hours", cfg.DomainRecheckIntervalHours},
		{"onboarding.completed", cfg.OnboardingCompleted},
	}

	for _, e := range entries {
		valueJSON, err := json.Marshal(e.value)
		if err != nil {
			return fmt.Errorf("marshaling config key %q: %w", e.key, err)
		}

		_, err = r.pool.Exec(ctx,
			`INSERT INTO global_config (key, value, updated_at)
			 VALUES (@key, @value, now())
			 ON CONFLICT (key) DO UPDATE SET value = @value, updated_at = now()`,
			pgx.NamedArgs{
				"key":   e.key,
				"value": valueJSON,
			},
		)
		if err != nil {
			return fmt.Errorf("upserting config key %q: %w", e.key, err)
		}
	}

	return nil
}

func mapConfigValue(cfg *domain.GlobalConfig, key string, value json.RawMessage) error {
	switch key {
	case "email.default_retry_count":
		return json.Unmarshal(value, &cfg.DefaultRetryCount)
	case "email.retry_backoff_base_seconds":
		return json.Unmarshal(value, &cfg.RetryBackoffBaseSeconds)
	case "email.log_retention_days":
		return json.Unmarshal(value, &cfg.LogRetentionDays)
	case "bounce.alert_threshold_percent":
		return json.Unmarshal(value, &cfg.BounceAlertThresholdPercent)
	case "complaint.alert_threshold_percent":
		return json.Unmarshal(value, &cfg.ComplaintAlertThresholdPercent)
	case "domain.recheck_interval_hours":
		return json.Unmarshal(value, &cfg.DomainRecheckIntervalHours)
	case "onboarding.completed":
		return json.Unmarshal(value, &cfg.OnboardingCompleted)
	default:
		// Unknown keys (e.g. oidc.*) are intentionally ignored.
		return nil
	}
}

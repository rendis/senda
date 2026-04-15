package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderRateLimiter implements port.RateLimiter using PG token buckets.
type ProviderRateLimiter struct {
	pool *pgxpool.Pool
}

// NewProviderRateLimiter creates a new rate limiter backed by the given pool.
func NewProviderRateLimiter(pool *pgxpool.Pool) *ProviderRateLimiter {
	return &ProviderRateLimiter{pool: pool}
}

// AcquireBurst atomically reserves up to requested tokens for the given adapter.
func (r *ProviderRateLimiter) AcquireBurst(ctx context.Context, adapterID uuid.UUID, requested int) (int, error) {
	var reserved int
	err := r.pool.QueryRow(ctx, "SELECT take_send_token_burst($1, $2)", adapterID, requested).Scan(&reserved)
	if err != nil {
		return 0, fmt.Errorf("rate limiter acquire burst for adapter %s: %w", adapterID, err)
	}
	return reserved, nil
}

// TryAcquire atomically attempts to consume one token from the bucket for the given adapter.
// Returns true if a token was available, false otherwise.
func (r *ProviderRateLimiter) TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error) {
	reserved, err := r.AcquireBurst(ctx, adapterID, 1)
	if err != nil {
		return false, err
	}
	return reserved > 0, nil
}

// SyncBucket creates or updates the token bucket for the given adapter.
func (r *ProviderRateLimiter) SyncBucket(ctx context.Context, adapterID uuid.UUID, maxPerSecond int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO token_buckets (adapter_id, tokens, max_tokens, refill_rate, last_refill)
		 VALUES ($1, $2::float, $2::int, $2::float, now())
		 ON CONFLICT (adapter_id) DO UPDATE
		 SET max_tokens = $2::int, refill_rate = $2::float`,
		adapterID, maxPerSecond,
	)
	if err != nil {
		return fmt.Errorf("rate limiter sync bucket for adapter %s: %w", adapterID, err)
	}
	return nil
}

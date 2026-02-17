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

// TryAcquire atomically attempts to consume one token from the bucket for the given adapter.
// Returns true if a token was available, false otherwise.
func (r *ProviderRateLimiter) TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, "SELECT take_send_token($1)", adapterID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("rate limiter try acquire for adapter %s: %w", adapterID, err)
	}
	return allowed, nil
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

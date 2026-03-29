package pgcache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/pkg/apperr"
)

// PGCache implements port.Cache using a PostgreSQL UNLOGGED table.
type PGCache struct {
	pool *pgxpool.Pool
}

// NewPGCache creates a new PGCache backed by the given connection pool.
func NewPGCache(pool *pgxpool.Pool) *PGCache {
	return &PGCache{pool: pool}
}

// Get retrieves a cached value by key. Returns apperr.NotFound on cache miss.
func (c *PGCache) Get(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := c.pool.QueryRow(ctx,
		"SELECT value FROM cache WHERE key = $1 AND expires_at > now()",
		key,
	).Scan(&value)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperr.NotFound("cache miss: %s", key)
		}
		return nil, fmt.Errorf("pgcache get %q: %w", key, err)
	}
	return value, nil
}

// Set stores a value with the given TTL, upserting if the key already exists.
// expires_at is computed by PostgreSQL to avoid clock-skew issues.
func (c *PGCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	_, err := c.pool.Exec(ctx,
		`INSERT INTO cache (key, value, expires_at)
		 VALUES ($1, $2, now() + $3::interval)
		 ON CONFLICT (key) DO UPDATE
		 SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, created_at = now()`,
		key, value, fmt.Sprintf("%d seconds", int(ttl.Seconds())),
	)
	if err != nil {
		return fmt.Errorf("pgcache set %q: %w", key, err)
	}
	return nil
}

// Delete removes a cached value by key.
func (c *PGCache) Delete(ctx context.Context, key string) error {
	_, err := c.pool.Exec(ctx, "DELETE FROM cache WHERE key = $1", key)
	if err != nil {
		return fmt.Errorf("pgcache delete %q: %w", key, err)
	}
	return nil
}

// DeletePattern removes all cached values matching a glob pattern.
// Converts glob wildcards (*) to SQL LIKE wildcards (%).
func (c *PGCache) DeletePattern(ctx context.Context, pattern string) error {
	likePattern := strings.ReplaceAll(pattern, "*", "%")
	_, err := c.pool.Exec(ctx, "DELETE FROM cache WHERE key LIKE $1", likePattern)
	if err != nil {
		return fmt.Errorf("pgcache delete pattern %q: %w", pattern, err)
	}
	return nil
}


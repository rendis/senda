package port

import (
	"context"
	"time"
)

// Cache provides key-value caching via PG UNLOGGED table.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	DeletePattern(ctx context.Context, pattern string) error // for global invalidation
}

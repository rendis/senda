package port

import (
	"context"

	"github.com/google/uuid"
)

// RateLimiter controls provider send rate using token bucket algorithm.
type RateLimiter interface {
	TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error)
	SyncBucket(ctx context.Context, adapterID uuid.UUID, maxPerSecond int) error
}

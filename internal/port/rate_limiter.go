package port

import (
	"context"

	"github.com/google/uuid"
)

// RateLimiter controls provider send rate using token bucket algorithm.
type RateLimiter interface {
	AcquireBurst(ctx context.Context, adapterID uuid.UUID, requested int) (int, error)
	TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error)
	SyncBucket(ctx context.Context, adapterID uuid.UUID, maxPerSecond int) error
}

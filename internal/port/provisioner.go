package port

import (
	"context"

	"github.com/google/uuid"
)

// Deprovisioner cleans up provider-side resources when an adapter is deleted.
type Deprovisioner interface {
	Deprovision(ctx context.Context, adapterID uuid.UUID) error
}

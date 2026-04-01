package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

// ProvisioningStepRepo implements port.ProvisioningStepStore using PostgreSQL.
type ProvisioningStepRepo struct {
	pool *pgxpool.Pool
}

func NewProvisioningStepRepo(pool *pgxpool.Pool) *ProvisioningStepRepo {
	return &ProvisioningStepRepo{pool: pool}
}

func (r *ProvisioningStepRepo) InitSteps(ctx context.Context, adapterID uuid.UUID) error {
	return r.initStepDefs(ctx, adapterID, domain.ProvisionStepDefs)
}

func (r *ProvisioningStepRepo) InitDeprovisionSteps(ctx context.Context, adapterID uuid.UUID) error {
	return r.initStepDefs(ctx, adapterID, domain.DeprovisionStepDefs)
}

func (r *ProvisioningStepRepo) initStepDefs(ctx context.Context, adapterID uuid.UUID, defs []struct {
	Name  domain.ProvisionStepName
	Order int
}) error {
	query := `INSERT INTO adapter_provisioning_steps (adapter_id, step_name, step_order)
		VALUES (@adapter_id, @step_name, @step_order)
		ON CONFLICT (adapter_id, step_name) DO NOTHING`

	batch := &pgx.Batch{}
	for _, s := range defs {
		batch.Queue(query, pgx.NamedArgs{
			"adapter_id": adapterID,
			"step_name":  s.Name,
			"step_order": s.Order,
		})
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range defs {
		if _, err := br.Exec(); err != nil {
			if appErr := classifyPgError(err); appErr != nil {
				return appErr
			}
			return fmt.Errorf("init provisioning steps: %w", err)
		}
	}
	return nil
}

func (r *ProvisioningStepRepo) ListByAdapter(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterProvisioningStep, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, adapter_id, step_name, step_order, status, resource_name, resource_arn,
		        error_message, started_at, completed_at, created_at, updated_at
		 FROM adapter_provisioning_steps
		 WHERE adapter_id = @adapter_id
		 ORDER BY step_order`,
		pgx.NamedArgs{"adapter_id": adapterID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing provisioning steps: %w", err)
	}

	steps, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.AdapterProvisioningStep])
	if err != nil {
		return nil, fmt.Errorf("collecting provisioning steps: %w", err)
	}
	return steps, nil
}

func (r *ProvisioningStepRepo) MarkCompleted(ctx context.Context, stepID uuid.UUID, resourceName, resourceARN *string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE adapter_provisioning_steps
		 SET status = 'completed',
		     resource_name = @resource_name,
		     resource_arn = @resource_arn,
		     error_message = NULL,
		     completed_at = now(),
		     updated_at = now()
		 WHERE id = @id`,
		pgx.NamedArgs{
			"id":            stepID,
			"resource_name": resourceName,
			"resource_arn":  resourceARN,
		},
	)
	if err != nil {
		return fmt.Errorf("marking step completed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("provisioning step %s not found", stepID)
	}
	return nil
}

func (r *ProvisioningStepRepo) MarkFailed(ctx context.Context, stepID uuid.UUID, errMsg string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE adapter_provisioning_steps
		 SET status = 'failed',
		     error_message = @error_message,
		     completed_at = NULL,
		     updated_at = now()
		 WHERE id = @id`,
		pgx.NamedArgs{
			"id":            stepID,
			"error_message": errMsg,
		},
	)
	if err != nil {
		return fmt.Errorf("marking step failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("provisioning step %s not found", stepID)
	}
	return nil
}

func (r *ProvisioningStepRepo) ResetFailed(ctx context.Context, adapterID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE adapter_provisioning_steps
		 SET status = 'pending',
		     error_message = NULL,
		     started_at = NULL,
		     completed_at = NULL,
		     updated_at = now()
		 WHERE adapter_id = @adapter_id AND status = 'failed'`,
		pgx.NamedArgs{"adapter_id": adapterID},
	)
	if err != nil {
		return fmt.Errorf("resetting failed steps: %w", err)
	}
	return nil
}

func (r *ProvisioningStepRepo) DeleteByAdapter(ctx context.Context, adapterID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM adapter_provisioning_steps WHERE adapter_id = @adapter_id`,
		pgx.NamedArgs{"adapter_id": adapterID},
	)
	if err != nil {
		return fmt.Errorf("deleting provisioning steps: %w", err)
	}
	return nil
}

// Ensure interface compliance.
var _ interface {
	InitSteps(ctx context.Context, adapterID uuid.UUID) error
	InitDeprovisionSteps(ctx context.Context, adapterID uuid.UUID) error
	ListByAdapter(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterProvisioningStep, error)
	MarkCompleted(ctx context.Context, stepID uuid.UUID, resourceName, resourceARN *string) error
	MarkFailed(ctx context.Context, stepID uuid.UUID, errMsg string) error
	ResetFailed(ctx context.Context, adapterID uuid.UUID) error
	DeleteByAdapter(ctx context.Context, adapterID uuid.UUID) error
} = (*ProvisioningStepRepo)(nil)

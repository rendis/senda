package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
)

// AdapterRepo implements port.AdapterStore using PostgreSQL.
type AdapterRepo struct {
	pool *pgxpool.Pool
}

// NewAdapterRepo creates a new AdapterRepo.
func NewAdapterRepo(pool *pgxpool.Pool) *AdapterRepo {
	return &AdapterRepo{pool: pool}
}

func (r *AdapterRepo) Create(ctx context.Context, adapter *domain.Adapter) error { //nolint:dupl // structurally similar to TemplateRepo.CreateType
	row := r.pool.QueryRow(ctx,
		`INSERT INTO adapters (id, name, workspace_id, adapter_type, config_encrypted, is_default, rate_limit_per_second, config_meta)
		 VALUES (@id, @name, @workspace_id, @adapter_type, @config_encrypted, @is_default, @rate_limit_per_second, @config_meta)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":                   adapter.ID,
			"name":                 adapter.Name,
			"workspace_id":         adapter.WorkspaceID,
			"adapter_type":         adapter.AdapterType,
			"config_encrypted":     adapter.ConfigEncrypted,
			"is_default":           adapter.IsDefault,
			"rate_limit_per_second": adapter.RateLimitPerSecond,
			"config_meta":          coalesceStringMap(adapter.ConfigMeta),
		},
	)

	if err := row.Scan(&adapter.CreatedAt, &adapter.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting adapter: %w", err)
	}

	return nil
}

func (r *AdapterRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, adapter_type, config_encrypted, is_default, rate_limit_per_second,
		        config_meta, created_at, updated_at, deleted_at
		 FROM adapters
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanAdapter(row)
}

func (r *AdapterRepo) Update(ctx context.Context, adapter *domain.Adapter) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE adapters
		 SET name = @name,
		     adapter_type = @adapter_type,
		     config_encrypted = @config_encrypted,
		     is_default = @is_default,
		     rate_limit_per_second = @rate_limit_per_second,
		     config_meta = @config_meta,
		     updated_at = now()
		 WHERE id = @id AND deleted_at IS NULL
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":                   adapter.ID,
			"name":                 adapter.Name,
			"adapter_type":         adapter.AdapterType,
			"config_encrypted":     adapter.ConfigEncrypted,
			"is_default":           adapter.IsDefault,
			"rate_limit_per_second": adapter.RateLimitPerSecond,
			"config_meta":          coalesceStringMap(adapter.ConfigMeta),
		},
	)

	if err := row.Scan(&adapter.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("adapter %s not found", adapter.ID)
		}
		return fmt.Errorf("updating adapter: %w", err)
	}

	return nil
}

func (r *AdapterRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE adapters SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting adapter: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("adapter %s not found", id)
	}
	return nil
}

func (r *AdapterRepo) ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Adapter, error) {
	nonNull, includeGlobal := splitChain(scopes)

	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, name, adapter_type, config_encrypted, is_default, rate_limit_per_second,
		        config_meta, created_at, updated_at, deleted_at
		 FROM adapters
		 WHERE (workspace_id = ANY(@scopes) OR (@include_global::bool AND workspace_id IS NULL))
		   AND deleted_at IS NULL
		 ORDER BY id DESC`,
		pgx.NamedArgs{
			"scopes":         nonNull,
			"include_global": includeGlobal,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("listing adapters in chain: %w", err)
	}

	adapters, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Adapter])
	if err != nil {
		return nil, fmt.Errorf("collecting adapters: %w", err)
	}

	return adapters, nil
}

func (r *AdapterRepo) ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, err
	}

	fetchLimit := limit + 1

	var rows pgx.Rows
	if workspaceID == nil {
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, name, adapter_type, config_encrypted, is_default, rate_limit_per_second,
				        config_meta, created_at, updated_at, deleted_at
				 FROM adapters
				 WHERE workspace_id IS NULL AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, name, adapter_type, config_encrypted, is_default, rate_limit_per_second,
				        config_meta, created_at, updated_at, deleted_at
				 FROM adapters
				 WHERE workspace_id IS NULL AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"limit": fetchLimit},
			)
		}
	} else {
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, name, adapter_type, config_encrypted, is_default, rate_limit_per_second,
				        config_meta, created_at, updated_at, deleted_at
				 FROM adapters
				 WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"workspace_id": *workspaceID, "after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, name, adapter_type, config_encrypted, is_default, rate_limit_per_second,
				        config_meta, created_at, updated_at, deleted_at
				 FROM adapters
				 WHERE workspace_id = @workspace_id AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"workspace_id": *workspaceID, "limit": fetchLimit},
			)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("listing adapters by workspace: %w", err)
	}

	adapters, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Adapter])
	if err != nil {
		return nil, fmt.Errorf("collecting adapters: %w", err)
	}

	result := &port.PageResult[domain.Adapter]{
		Items:   adapters,
		HasMore: len(adapters) > limit,
	}
	if result.HasMore {
		result.Items = adapters[:limit]
		result.NextCursor = EncodeCursor(adapters[limit-1].ID)
	}

	return result, nil
}

func scanAdapter(row pgx.Row) (*domain.Adapter, error) {
	var a domain.Adapter
	err := row.Scan(
		&a.ID, &a.WorkspaceID, &a.Name, &a.AdapterType, &a.ConfigEncrypted,
		&a.IsDefault, &a.RateLimitPerSecond, &a.ConfigMeta,
		&a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("adapter not found")
		}
		return nil, fmt.Errorf("scanning adapter: %w", err)
	}
	return &a, nil
}

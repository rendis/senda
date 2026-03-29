package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
)

// TenantRepo implements port.TenantStore using PostgreSQL.
type TenantRepo struct {
	pool *pgxpool.Pool
}

// NewTenantRepo creates a new TenantRepo.
func NewTenantRepo(pool *pgxpool.Pool) *TenantRepo {
	return &TenantRepo{pool: pool}
}

func (r *TenantRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO tenants (id, code, name)
		 VALUES (@id, @code, @name)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":   tenant.ID,
			"code": tenant.Code,
			"name": tenant.Name,
		},
	)

	if err := row.Scan(&tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting tenant: %w", err)
	}

	return nil
}

func (r *TenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, code, name, created_at, updated_at, deleted_at
		 FROM tenants
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanTenant(row)
}

func (r *TenantRepo) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, code, name, created_at, updated_at, deleted_at
		 FROM tenants
		 WHERE code = @code AND deleted_at IS NULL`,
		pgx.NamedArgs{"code": code},
	)

	return scanTenant(row)
}

func (r *TenantRepo) List(ctx context.Context, opts port.ListOptions) ([]*domain.Tenant, string, error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, "", err
	}

	fetchLimit := limit + 1
	args := pgx.NamedArgs{"limit": fetchLimit}

	var qb strings.Builder
	qb.WriteString(`SELECT id, code, name, created_at, updated_at, deleted_at FROM tenants WHERE deleted_at IS NULL`)

	if afterID != nil {
		qb.WriteString(` AND id < @after_id`)
		args["after_id"] = *afterID
	}

	if opts.Search != "" {
		qb.WriteString(` AND (name ILIKE @search OR code ILIKE @search)`)
		args["search"] = "%" + opts.Search + "%"
	}

	qb.WriteString(` ORDER BY id DESC LIMIT @limit`)

	rows, err := r.pool.Query(ctx, qb.String(), args)
	if err != nil {
		return nil, "", fmt.Errorf("listing tenants: %w", err)
	}

	tenants, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Tenant])
	if err != nil {
		return nil, "", fmt.Errorf("collecting tenants: %w", err)
	}

	var nextCursor string
	if len(tenants) > limit {
		tenants = tenants[:limit]
		nextCursor = EncodeCursor(tenants[limit-1].ID)
	}

	return tenants, nextCursor, nil
}

func (r *TenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE tenants
		 SET name = @name, updated_at = now()
		 WHERE id = @id AND deleted_at IS NULL
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":   tenant.ID,
			"name": tenant.Name,
		},
	)

	if err := row.Scan(&tenant.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("tenant %s not found", tenant.ID)
		}
		return fmt.Errorf("updating tenant: %w", err)
	}

	return nil
}

func (r *TenantRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tenants SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("tenant %s not found", id)
	}
	return nil
}

func (r *TenantRepo) Purge(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM tenants WHERE id = @id`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("purging tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("tenant %s not found", id)
	}
	return nil
}

func scanTenant(row pgx.Row) (*domain.Tenant, error) {
	var t domain.Tenant
	err := row.Scan(&t.ID, &t.Code, &t.Name, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("tenant not found")
		}
		return nil, fmt.Errorf("scanning tenant: %w", err)
	}
	return &t, nil
}

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/apperr"
)

// WorkspaceRepo implements port.WorkspaceStore using PostgreSQL.
type WorkspaceRepo struct {
	pool *pgxpool.Pool
}

// NewWorkspaceRepo creates a new WorkspaceRepo.
func NewWorkspaceRepo(pool *pgxpool.Pool) *WorkspaceRepo {
	return &WorkspaceRepo{pool: pool}
}

func (r *WorkspaceRepo) Create(ctx context.Context, ws *domain.Workspace) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO workspaces (id, tenant_id, code, name, is_system, open_tracking_enabled, default_locale)
		 VALUES (@id, @tenant_id, @code, @name, @is_system, @open_tracking_enabled, @default_locale)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":                    ws.ID,
			"tenant_id":            ws.TenantID,
			"code":                 ws.Code,
			"name":                 ws.Name,
			"is_system":            ws.IsSystem,
			"open_tracking_enabled": ws.OpenTrackingEnabled,
			"default_locale":       ws.DefaultLocale,
		},
	)

	if err := row.Scan(&ws.CreatedAt, &ws.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperr.Conflict("workspace with code %q already exists for this tenant", ws.Code)
		}
		return fmt.Errorf("inserting workspace: %w", err)
	}

	return nil
}

func (r *WorkspaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, code, name, is_system, open_tracking_enabled, default_locale,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, code, name, is_system, open_tracking_enabled, default_locale,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE tenant_id = @tenant_id AND code = @code AND deleted_at IS NULL`,
		pgx.NamedArgs{"tenant_id": tenantID, "code": code},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, code, name, is_system, open_tracking_enabled, default_locale,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE tenant_id = @tenant_id AND is_system = true AND deleted_at IS NULL`,
		pgx.NamedArgs{"tenant_id": tenantID},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, "", err
	}

	fetchLimit := limit + 1
	args := pgx.NamedArgs{"tenant_id": tenantID, "limit": fetchLimit}

	var qb strings.Builder
	qb.WriteString(`SELECT id, tenant_id, code, name, is_system, open_tracking_enabled, default_locale, created_at, updated_at, deleted_at FROM workspaces WHERE tenant_id = @tenant_id AND deleted_at IS NULL`)

	if afterID != nil {
		qb.WriteString(` AND id < @after_id`)
		args["after_id"] = *afterID
	}

	if opts.Search != "" {
		qb.WriteString(` AND (name ILIKE @search OR code ILIKE @search)`)
		args["search"] = "%" + opts.Search + "%"
	}

	qb.WriteString(` ORDER BY is_system DESC, id DESC LIMIT @limit`)

	rows, err := r.pool.Query(ctx, qb.String(), args)
	if err != nil {
		return nil, "", fmt.Errorf("listing workspaces: %w", err)
	}

	workspaces, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Workspace])
	if err != nil {
		return nil, "", fmt.Errorf("collecting workspaces: %w", err)
	}

	var nextCursor string
	if len(workspaces) > limit {
		workspaces = workspaces[:limit]
		nextCursor = EncodeCursor(workspaces[limit-1].ID)
	}

	return workspaces, nextCursor, nil
}

func (r *WorkspaceRepo) Update(ctx context.Context, ws *domain.Workspace) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE workspaces
		 SET name = @name,
		     open_tracking_enabled = @open_tracking_enabled,
		     default_locale = @default_locale,
		     updated_at = now()
		 WHERE id = @id AND deleted_at IS NULL
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":                    ws.ID,
			"name":                 ws.Name,
			"open_tracking_enabled": ws.OpenTrackingEnabled,
			"default_locale":       ws.DefaultLocale,
		},
	)

	if err := row.Scan(&ws.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("workspace %s not found", ws.ID)
		}
		return fmt.Errorf("updating workspace: %w", err)
	}

	return nil
}

func (r *WorkspaceRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspaces SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting workspace: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("workspace %s not found", id)
	}
	return nil
}

func scanWorkspace(row pgx.Row) (*domain.Workspace, error) {
	var ws domain.Workspace
	err := row.Scan(
		&ws.ID, &ws.TenantID, &ws.Code, &ws.Name, &ws.IsSystem,
		&ws.OpenTrackingEnabled, &ws.DefaultLocale,
		&ws.CreatedAt, &ws.UpdatedAt, &ws.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("workspace not found")
		}
		return nil, fmt.Errorf("scanning workspace: %w", err)
	}
	return &ws, nil
}

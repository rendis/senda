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
		`INSERT INTO workspaces (
		    id, tenant_id, code, name, is_system, open_tracking_enabled, default_locale,
		    allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors
		)
		 VALUES (
		    @id, @tenant_id, @code, @name, @is_system, @open_tracking_enabled, @default_locale,
		    @allow_workspace_local_templates, @allow_workspace_inherited_template_forks, @allow_workspace_local_injectors
		)
		 RETURNING is_active, created_at, updated_at`,
		pgx.NamedArgs{
			"id":                              ws.ID,
			"tenant_id":                       ws.TenantID,
			"code":                            ws.Code,
			"name":                            ws.Name,
			"is_system":                       ws.IsSystem,
			"open_tracking_enabled":           ws.OpenTrackingEnabled,
			"default_locale":                  ws.DefaultLocale,
			"allow_workspace_local_templates": effectiveWorkspacePolicyValue(ws.AllowWorkspaceLocalTemplates, ws.WorkspacePoliciesInitialized),
			"allow_workspace_inherited_template_forks": effectiveWorkspacePolicyValue(ws.AllowWorkspaceInheritedTemplateForks, ws.WorkspacePoliciesInitialized),
			"allow_workspace_local_injectors":          effectiveWorkspacePolicyValue(ws.AllowWorkspaceLocalInjectors, ws.WorkspacePoliciesInitialized),
		},
	)

	if err := row.Scan(&ws.IsActive, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting workspace: %w", err)
	}
	ws.WorkspacePoliciesInitialized = true

	return nil
}

func (r *WorkspaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, code, name, is_system, is_active, open_tracking_enabled, default_locale,
		        allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, code, name, is_system, is_active, open_tracking_enabled, default_locale,
		        allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE tenant_id = @tenant_id AND code = @code AND deleted_at IS NULL`,
		pgx.NamedArgs{"tenant_id": tenantID, "code": code},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, code, name, is_system, is_active, open_tracking_enabled, default_locale,
		        allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
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
	qb.WriteString(`SELECT id, tenant_id, code, name, is_system, is_active, open_tracking_enabled, default_locale,
	allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
	created_at, updated_at, deleted_at FROM workspaces WHERE tenant_id = @tenant_id AND deleted_at IS NULL`)

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
	for _, workspace := range workspaces {
		workspace.WorkspacePoliciesInitialized = true
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
		     is_active = @is_active,
		     open_tracking_enabled = @open_tracking_enabled,
		     default_locale = @default_locale,
		     allow_workspace_local_templates = @allow_workspace_local_templates,
		     allow_workspace_inherited_template_forks = @allow_workspace_inherited_template_forks,
		     allow_workspace_local_injectors = @allow_workspace_local_injectors,
		     updated_at = now()
		 WHERE id = @id AND deleted_at IS NULL
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":                              ws.ID,
			"name":                            ws.Name,
			"is_active":                       ws.IsActive,
			"open_tracking_enabled":           ws.OpenTrackingEnabled,
			"default_locale":                  ws.DefaultLocale,
			"allow_workspace_local_templates": effectiveWorkspacePolicyValue(ws.AllowWorkspaceLocalTemplates, ws.WorkspacePoliciesInitialized),
			"allow_workspace_inherited_template_forks": effectiveWorkspacePolicyValue(ws.AllowWorkspaceInheritedTemplateForks, ws.WorkspacePoliciesInitialized),
			"allow_workspace_local_injectors":          effectiveWorkspacePolicyValue(ws.AllowWorkspaceLocalInjectors, ws.WorkspacePoliciesInitialized),
		},
	)

	if err := row.Scan(&ws.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("workspace %s not found", ws.ID)
		}
		return fmt.Errorf("updating workspace: %w", err)
	}
	ws.WorkspacePoliciesInitialized = true

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

func (r *WorkspaceRepo) ExistsActiveByTenantCode(ctx context.Context, tenantCode string, workspaceCodes []string) (map[string]bool, error) {
	result := make(map[string]bool, len(workspaceCodes))
	if len(workspaceCodes) == 0 {
		return result, nil
	}

	requested := uniqueWorkspaceCodes(workspaceCodes)
	for _, code := range requested {
		result[code] = false
	}

	rows, err := r.pool.Query(ctx, `
		WITH requested(code) AS (
			SELECT DISTINCT unnest($2::text[])
		)
		SELECT requested.code,
		       (w.id IS NOT NULL) AS exists_active
		FROM requested
		LEFT JOIN tenants t
		  ON t.code = $1
		 AND t.deleted_at IS NULL
		LEFT JOIN workspaces w
		  ON w.tenant_id = t.id
		 AND w.code = requested.code
		 AND w.deleted_at IS NULL
		 AND w.is_active = true
		ORDER BY requested.code`, tenantCode, requested)
	if err != nil {
		return nil, fmt.Errorf("querying workspace existence by tenant code: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var exists bool
		if err := rows.Scan(&code, &exists); err != nil {
			return nil, fmt.Errorf("scanning workspace existence row: %w", err)
		}
		result[code] = exists
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating workspace existence rows: %w", err)
	}

	return result, nil
}

func scanWorkspace(row pgx.Row) (*domain.Workspace, error) {
	var ws domain.Workspace
	err := row.Scan(
		&ws.ID, &ws.TenantID, &ws.Code, &ws.Name, &ws.IsSystem, &ws.IsActive,
		&ws.OpenTrackingEnabled, &ws.DefaultLocale,
		&ws.AllowWorkspaceLocalTemplates, &ws.AllowWorkspaceInheritedTemplateForks, &ws.AllowWorkspaceLocalInjectors,
		&ws.CreatedAt, &ws.UpdatedAt, &ws.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("workspace not found")
		}
		return nil, fmt.Errorf("scanning workspace: %w", err)
	}
	ws.WorkspacePoliciesInitialized = true
	return &ws, nil
}

func effectiveWorkspacePolicyValue(value bool, initialized bool) bool {
	if initialized {
		return value
	}
	return true
}

func uniqueWorkspaceCodes(workspaceCodes []string) []string {
	seen := make(map[string]struct{}, len(workspaceCodes))
	result := make([]string, 0, len(workspaceCodes))
	for _, code := range workspaceCodes {
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

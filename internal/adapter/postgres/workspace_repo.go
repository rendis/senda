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
	return insertWorkspace(ctx, r.pool, ws)
}

func (r *WorkspaceRepo) CreateLogicalPair(ctx context.Context, prod *domain.Workspace, test *domain.Workspace) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning logical workspace transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := insertWorkspace(ctx, tx, prod); err != nil {
		return err
	}
	if err := insertWorkspace(ctx, tx, test); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing logical workspace transaction: %w", err)
	}
	return nil
}

func (r *WorkspaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, logical_workspace_id, tenant_id, code, name, environment, is_system, is_active, open_tracking_enabled, default_locale, test_recipient_mode, test_recipient_addresses,
		        allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, logical_workspace_id, tenant_id, code, name, environment, is_system, is_active, open_tracking_enabled, default_locale, test_recipient_mode, test_recipient_addresses,
		        allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE tenant_id = @tenant_id AND code = @code AND environment = @environment AND deleted_at IS NULL`,
		pgx.NamedArgs{"tenant_id": tenantID, "code": code, "environment": environment},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID, environment domain.Environment) (*domain.Workspace, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, logical_workspace_id, tenant_id, code, name, environment, is_system, is_active, open_tracking_enabled, default_locale, test_recipient_mode, test_recipient_addresses,
		        allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
		        created_at, updated_at, deleted_at
		 FROM workspaces
		 WHERE tenant_id = @tenant_id AND is_system = true AND environment = @environment AND deleted_at IS NULL`,
		pgx.NamedArgs{"tenant_id": tenantID, "environment": environment},
	)

	return scanWorkspace(row)
}

func (r *WorkspaceRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, "", err
	}

	fetchLimit := limit + 1
	args := pgx.NamedArgs{"tenant_id": tenantID, "environment": environment, "limit": fetchLimit}

	var qb strings.Builder
	qb.WriteString(`SELECT id, logical_workspace_id, tenant_id, code, name, environment, is_system, is_active, open_tracking_enabled, default_locale, test_recipient_mode, test_recipient_addresses,
	allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
	created_at, updated_at, deleted_at FROM workspaces WHERE tenant_id = @tenant_id AND environment = @environment AND deleted_at IS NULL`)

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

	workspaces, err := pgx.CollectRows(rows, scanWorkspaceCollectableRow)
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

func (r *WorkspaceRepo) UpdateShared(ctx context.Context, tenantID uuid.UUID, currentCode, nextCode, nextName string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspaces
		 SET code = @next_code,
		     name = @next_name,
		     updated_at = now()
		 WHERE tenant_id = @tenant_id AND code = @current_code AND deleted_at IS NULL`,
		pgx.NamedArgs{
			"tenant_id":    tenantID,
			"current_code": currentCode,
			"next_code":    nextCode,
			"next_name":    nextName,
		},
	)
	if err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("updating shared workspace fields: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("workspace %s not found", currentCode)
	}
	return nil
}

func (r *WorkspaceRepo) Update(ctx context.Context, ws *domain.Workspace) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE workspaces
		 SET name = @name,
		     is_active = @is_active,
		     open_tracking_enabled = @open_tracking_enabled,
		     default_locale = @default_locale,
		     test_recipient_mode = @test_recipient_mode,
		     test_recipient_addresses = @test_recipient_addresses,
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
			"test_recipient_mode":             effectiveTestRecipientMode(ws.TestRecipientMode),
			"test_recipient_addresses":        domain.NormalizeRecipientAddresses(ws.TestRecipientAddresses),
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

func (r *WorkspaceRepo) SoftDeleteLogical(ctx context.Context, tenantID uuid.UUID, code string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspaces
		 SET deleted_at = now()
		 WHERE tenant_id = @tenant_id AND code = @code AND deleted_at IS NULL`,
		pgx.NamedArgs{"tenant_id": tenantID, "code": code},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting logical workspace: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("workspace %s not found", code)
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

func (r *WorkspaceRepo) ExistsActiveByTenantCode(ctx context.Context, tenantCode string, workspaceCodes []string, environment domain.Environment) (map[string]bool, error) {
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
		 AND w.environment = $3
		 AND w.deleted_at IS NULL
		 AND w.is_active = true
		ORDER BY requested.code`, tenantCode, requested, environment)
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

type workspaceQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertWorkspace(ctx context.Context, q workspaceQuerier, ws *domain.Workspace) error {
	if ws.LogicalWorkspaceID == uuid.Nil {
		ws.LogicalWorkspaceID = ws.ID
	}
	if !ws.Environment.Valid() {
		ws.Environment = domain.EnvironmentProd
	}

	row := q.QueryRow(ctx,
		`INSERT INTO workspaces (
		    id, logical_workspace_id, tenant_id, code, name, environment, is_system, open_tracking_enabled, default_locale, test_recipient_mode, test_recipient_addresses,
		    allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors
		)
		 VALUES (
		    @id, @logical_workspace_id, @tenant_id, @code, @name, @environment, @is_system, @open_tracking_enabled, @default_locale, @test_recipient_mode, @test_recipient_addresses,
		    @allow_workspace_local_templates, @allow_workspace_inherited_template_forks, @allow_workspace_local_injectors
		)
		 RETURNING is_active, created_at, updated_at`,
		pgx.NamedArgs{
			"id":                              ws.ID,
			"logical_workspace_id":            ws.LogicalWorkspaceID,
			"tenant_id":                       ws.TenantID,
			"code":                            ws.Code,
			"name":                            ws.Name,
			"environment":                     ws.Environment,
			"is_system":                       ws.IsSystem,
			"open_tracking_enabled":           ws.OpenTrackingEnabled,
			"default_locale":                  ws.DefaultLocale,
			"test_recipient_mode":             effectiveTestRecipientMode(ws.TestRecipientMode),
			"test_recipient_addresses":        domain.NormalizeRecipientAddresses(ws.TestRecipientAddresses),
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

func scanWorkspace(row pgx.Row) (*domain.Workspace, error) {
	return scanWorkspaceScanner(row)
}

func scanWorkspaceCollectableRow(row pgx.CollectableRow) (*domain.Workspace, error) {
	return scanWorkspaceScanner(row)
}

type workspaceScanner interface {
	Scan(dest ...any) error
}

func scanWorkspaceScanner(row workspaceScanner) (*domain.Workspace, error) {
	var ws domain.Workspace
	err := row.Scan(
		&ws.ID, &ws.LogicalWorkspaceID, &ws.TenantID, &ws.Code, &ws.Name, &ws.Environment, &ws.IsSystem, &ws.IsActive,
		&ws.OpenTrackingEnabled, &ws.DefaultLocale, &ws.TestRecipientMode, &ws.TestRecipientAddresses,
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

func effectiveTestRecipientMode(mode domain.TestRecipientMode) domain.TestRecipientMode {
	if mode.Valid() {
		return mode
	}
	return domain.TestRecipientModeReplace
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

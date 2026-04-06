package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

const whereWorkspaceID = ` AND workspace_id = @workspace_id`

// AdapterGrantRepo implements adapter-level workspace sharing persistence.
type AdapterGrantRepo struct {
	pool *pgxpool.Pool
}

// NewAdapterGrantRepo creates a new AdapterGrantRepo.
func NewAdapterGrantRepo(pool *pgxpool.Pool) *AdapterGrantRepo {
	return &AdapterGrantRepo{pool: pool}
}

func (r *AdapterGrantRepo) ListAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT workspace_id
		   FROM adapter_workspace_grants
		  WHERE adapter_id = @adapter_id
		  ORDER BY workspace_id`,
		pgx.NamedArgs{"adapter_id": adapterID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing adapter workspace grants: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (uuid.UUID, error) {
		var id uuid.UUID
		return id, row.Scan(&id)
	})
}

func (r *AdapterGrantRepo) ReplaceAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) error { //nolint:dupl // structurally similar to ReplaceIdentityWorkspaceGrants
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM adapter_workspace_grants WHERE adapter_id = @adapter_id`,
		pgx.NamedArgs{"adapter_id": adapterID},
	); err != nil {
		return fmt.Errorf("deleting adapter workspace grants: %w", err)
	}

	for _, workspaceID := range workspaceIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO adapter_workspace_grants (adapter_id, workspace_id)
			 VALUES (@adapter_id, @workspace_id)`,
			pgx.NamedArgs{"adapter_id": adapterID, "workspace_id": workspaceID},
		); err != nil {
			return fmt.Errorf("inserting adapter workspace grant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing adapter workspace grants: %w", err)
	}
	return nil
}

func (r *AdapterGrantRepo) HasAdapterWorkspaceGrant(ctx context.Context, adapterID, workspaceID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1
			  FROM adapter_workspace_grants
			 WHERE adapter_id = @adapter_id
			   AND workspace_id = @workspace_id
		)`,
		pgx.NamedArgs{"adapter_id": adapterID, "workspace_id": workspaceID},
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking adapter workspace grant: %w", err)
	}
	return exists, nil
}

// ListVisibleAdaptersForWorkspace returns workspace-owned adapters plus system-owned adapters
// shared either through adapter grants (Gmail) or identity grants (SES).
func (r *AdapterGrantRepo) ListVisibleAdaptersForWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, err
	}
	fetchLimit := limit + 1

	args := pgx.NamedArgs{
		"workspace_id": workspaceID,
		"limit":        fetchLimit,
	}
	query := `
WITH workspace_ctx AS (
	SELECT id, tenant_id
	  FROM workspaces
	 WHERE id = @workspace_id
	   AND deleted_at IS NULL
)
SELECT DISTINCT a.id, a.workspace_id, a.name, a.adapter_type, a.config_encrypted, a.is_default,
       a.rate_limit_per_second, a.config_meta, a.created_at, a.updated_at, a.deleted_at
  FROM adapters a
 CROSS JOIN workspace_ctx ws
 WHERE a.deleted_at IS NULL
   AND (
       a.workspace_id = ws.id
       OR (
           a.adapter_type = 'gmail'
           AND EXISTS (
               SELECT 1
                 FROM adapter_workspace_grants awg
                 JOIN workspaces owner_ws
                   ON owner_ws.id = a.workspace_id
                  AND owner_ws.deleted_at IS NULL
                WHERE awg.adapter_id = a.id
                  AND awg.workspace_id = ws.id
                  AND owner_ws.tenant_id = ws.tenant_id
                  AND owner_ws.is_system = true
           )
       )
       OR (
           a.adapter_type = 'ses'
           AND EXISTS (
               SELECT 1
                 FROM adapter_identities ai
                 JOIN adapter_identity_workspace_grants aiwg
                   ON aiwg.adapter_identity_id = ai.id
                 JOIN workspaces owner_ws
                   ON owner_ws.id = a.workspace_id
                  AND owner_ws.deleted_at IS NULL
                WHERE ai.adapter_id = a.id
                  AND aiwg.workspace_id = ws.id
                  AND owner_ws.tenant_id = ws.tenant_id
                  AND owner_ws.is_system = true
           )
       )
   )`
	if afterID != nil {
		query += ` AND a.id < @after_id`
		args["after_id"] = *afterID
	}
	query += `
 ORDER BY a.id DESC
 LIMIT @limit`

	rows, err := r.pool.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("listing visible adapters for workspace: %w", err)
	}

	adapters, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Adapter])
	if err != nil {
		return nil, fmt.Errorf("collecting visible adapters: %w", err)
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

// AdapterIdentityGrantRepo implements SES identity-level workspace sharing persistence.
type AdapterIdentityGrantRepo struct {
	pool *pgxpool.Pool
}

// NewAdapterIdentityGrantRepo creates a new AdapterIdentityGrantRepo.
func NewAdapterIdentityGrantRepo(pool *pgxpool.Pool) *AdapterIdentityGrantRepo {
	return &AdapterIdentityGrantRepo{pool: pool}
}

func (r *AdapterIdentityGrantRepo) ListIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT workspace_id
		   FROM adapter_identity_workspace_grants
		  WHERE adapter_identity_id = @identity_id
		  ORDER BY workspace_id`,
		pgx.NamedArgs{"identity_id": identityID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing identity workspace grants: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (uuid.UUID, error) {
		var id uuid.UUID
		return id, row.Scan(&id)
	})
}

func (r *AdapterIdentityGrantRepo) ReplaceIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) error { //nolint:dupl // structurally similar to ReplaceAdapterWorkspaceGrants
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM adapter_identity_workspace_grants WHERE adapter_identity_id = @identity_id`,
		pgx.NamedArgs{"identity_id": identityID},
	); err != nil {
		return fmt.Errorf("deleting identity workspace grants: %w", err)
	}

	for _, workspaceID := range workspaceIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO adapter_identity_workspace_grants (adapter_identity_id, workspace_id)
			 VALUES (@identity_id, @workspace_id)`,
			pgx.NamedArgs{"identity_id": identityID, "workspace_id": workspaceID},
		); err != nil {
			return fmt.Errorf("inserting identity workspace grant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing identity workspace grants: %w", err)
	}
	return nil
}

func (r *AdapterIdentityGrantRepo) HasIdentityWorkspaceGrant(ctx context.Context, identityID, workspaceID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1
			  FROM adapter_identity_workspace_grants
			 WHERE adapter_identity_id = @identity_id
			   AND workspace_id = @workspace_id
		)`,
		pgx.NamedArgs{"identity_id": identityID, "workspace_id": workspaceID},
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking identity workspace grant: %w", err)
	}
	return exists, nil
}

func (r *AdapterIdentityGrantRepo) ListGrantedIdentitiesForWorkspace(ctx context.Context, adapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ai.id, ai.adapter_id, ai.identity, ai.identity_type, ai.status, ai.sending_enabled, ai.is_default,
		        ai.display_name, ai.source, ai.last_synced_at,
		        0 AS granted_workspace_count,
		        ai.created_at, ai.updated_at
		   FROM adapter_identities ai
		   JOIN adapter_identity_workspace_grants aiwg
		     ON aiwg.adapter_identity_id = ai.id
		  WHERE ai.adapter_id = @adapter_id
		    AND aiwg.workspace_id = @workspace_id
		    AND ai.identity_type = 'email'
		  ORDER BY ai.is_default DESC, ai.identity ASC`,
		pgx.NamedArgs{"adapter_id": adapterID, "workspace_id": workspaceID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing granted identities for workspace: %w", err)
	}

	identities, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.AdapterIdentity])
	if err != nil {
		return nil, fmt.Errorf("collecting granted identities: %w", err)
	}
	return identities, nil
}

// TemplateTypeUsageRepo provides lightweight usage checks for shared resource revocation.
type TemplateTypeUsageRepo struct {
	pool *pgxpool.Pool
}

// NewTemplateTypeUsageRepo creates a new TemplateTypeUsageRepo.
func NewTemplateTypeUsageRepo(pool *pgxpool.Pool) *TemplateTypeUsageRepo {
	return &TemplateTypeUsageRepo{pool: pool}
}

func (r *TemplateTypeUsageRepo) CountTypesUsingAdapter(ctx context.Context, adapterID uuid.UUID, workspaceID *uuid.UUID) (int, error) {
	query := `
SELECT count(*)
  FROM template_types
 WHERE adapter_id = @adapter_id
   AND deleted_at IS NULL`
	args := pgx.NamedArgs{"adapter_id": adapterID}
	if workspaceID != nil {
		query += whereWorkspaceID
		args["workspace_id"] = *workspaceID
	}
	var count int
	if err := r.pool.QueryRow(ctx, query, args).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting template types using adapter: %w", err)
	}
	return count, nil
}

func (r *TemplateTypeUsageRepo) CountTypesUsingSenderIdentity(ctx context.Context, identityID uuid.UUID, workspaceID *uuid.UUID) (int, error) {
	query := `
SELECT count(*)
  FROM template_types
 WHERE sender_identity_id = @identity_id
   AND deleted_at IS NULL`
	args := pgx.NamedArgs{"identity_id": identityID}
	if workspaceID != nil {
		query += whereWorkspaceID
		args["workspace_id"] = *workspaceID
	}
	var count int
	if err := r.pool.QueryRow(ctx, query, args).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting template types using sender identity: %w", err)
	}
	return count, nil
}

var (
	_ port.AdapterGrantStore         = (*AdapterGrantRepo)(nil)
	_ port.AdapterIdentityGrantStore = (*AdapterIdentityGrantRepo)(nil)
	_ port.TemplateTypeUsageStore    = (*TemplateTypeUsageRepo)(nil)
)

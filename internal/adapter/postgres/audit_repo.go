package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// AuditRepo implements port.AuditLogStore using PostgreSQL.
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

func (r *AuditRepo) Append(ctx context.Context, entry *domain.AuditLog) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO audit_logs (id, member_id, member_email, action, resource_type, resource_id,
		        scope_type, tenant_id, workspace_id, changes, metadata)
		 VALUES (@id, @member_id, @member_email, @action, @resource_type, @resource_id,
		         @scope_type, @tenant_id, @workspace_id, @changes, @metadata)
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":            entry.ID,
			"member_id":     entry.ActorID,
			"member_email":  entry.ActorEmail,
			"action":        entry.Action,
			"resource_type": entry.EntityType,
			"resource_id":   entry.EntityID,
			"scope_type":    entry.ScopeType,
			"tenant_id":     entry.TenantID,
			"workspace_id":  entry.WorkspaceID,
			"changes":       entry.Changes,
			"metadata":      entry.Metadata,
		},
	)

	if err := row.Scan(&entry.CreatedAt); err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}

	return nil
}

func (r *AuditRepo) Query(ctx context.Context, filter port.AuditFilter, opts port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
	limit := NormalizeLimit(opts.Limit)
	fetchLimit := limit + 1

	where := "1=1"
	args := pgx.NamedArgs{}

	if filter.TenantID != nil {
		where += ` AND tenant_id = @tenant_id`
		args["tenant_id"] = *filter.TenantID
	}
	if filter.WorkspaceID != nil {
		where += ` AND workspace_id = @workspace_id`
		args["workspace_id"] = *filter.WorkspaceID
	}
	if filter.ActorID != nil {
		where += ` AND member_id = @member_id`
		args["member_id"] = *filter.ActorID
	}
	if filter.Action != nil {
		where += ` AND action = @action`
		args["action"] = *filter.Action
	}
	if filter.EntityType != nil {
		where += ` AND resource_type = @resource_type`
		args["resource_type"] = *filter.EntityType
	}
	if filter.Since != nil {
		where += ` AND created_at >= @since`
		args["since"] = *filter.Since
	}
	if filter.Until != nil {
		where += ` AND created_at < @until`
		args["until"] = *filter.Until
	}

	// Composite cursor for partitioned table
	if opts.Cursor != "" {
		cursorTime, cursorID, err := DecodeTimeCursor(opts.Cursor)
		if err != nil {
			return nil, err
		}
		where += ` AND (created_at, id) < (@cursor_time, @cursor_id)`
		args["cursor_time"] = cursorTime
		args["cursor_id"] = cursorID
	}

	args["limit"] = fetchLimit

	query := fmt.Sprintf(
		`SELECT id, member_id, member_email, action, resource_type, resource_id,
		        scope_type, tenant_id, workspace_id, changes, metadata, created_at
		 FROM audit_logs
		 WHERE %s
		 ORDER BY created_at DESC, id DESC
		 LIMIT @limit`, where,
	)

	rows, err := r.pool.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("querying audit logs: %w", err)
	}

	logs, err := pgx.CollectRows(rows, scanAuditRow)
	if err != nil {
		return nil, fmt.Errorf("collecting audit logs: %w", err)
	}

	result := &port.PageResult[domain.AuditLog]{}
	if len(logs) > limit {
		logs = logs[:limit]
		result.HasMore = true
		last := logs[limit-1]
		result.NextCursor = EncodeTimeCursor(last.CreatedAt, last.ID)
	}
	result.Items = logs

	return result, nil
}

func scanAuditRow(row pgx.CollectableRow) (*domain.AuditLog, error) {
	var a domain.AuditLog
	err := row.Scan(
		&a.ID, &a.ActorID, &a.ActorEmail, &a.Action, &a.EntityType, &a.EntityID,
		&a.ScopeType, &a.TenantID, &a.WorkspaceID, &a.Changes, &a.Metadata, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

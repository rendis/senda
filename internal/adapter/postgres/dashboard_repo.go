package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/port"
)

// DashboardRepo implements port.DashboardStore using PostgreSQL.
type DashboardRepo struct {
	pool *pgxpool.Pool
}

// NewDashboardRepo creates a new DashboardRepo.
func NewDashboardRepo(pool *pgxpool.Pool) *DashboardRepo {
	return &DashboardRepo{pool: pool}
}

// scopeFilter builds the WHERE clause fragment and populates named args for scope filtering.
func scopeFilter(p port.DashboardStatsParams, args pgx.NamedArgs) string {
	where := "created_at >= @since AND created_at < @until"
	args["since"] = p.Since
	args["until"] = p.Until

	if p.WorkspaceID != nil {
		where += " AND workspace_id = @workspace_id"
		args["workspace_id"] = *p.WorkspaceID
	} else if p.TenantID != nil {
		where += " AND tenant_id = @tenant_id"
		args["tenant_id"] = *p.TenantID
	}

	return where
}

func (r *DashboardRepo) GetTotals(ctx context.Context, p port.DashboardStatsParams) (*port.DashboardTotals, error) {
	args := pgx.NamedArgs{}
	where := scopeFilter(p, args)

	query := fmt.Sprintf(
		`SELECT count(*) FILTER (WHERE status IN ('sent','delivered','opened')) AS sent,
		        count(*) FILTER (WHERE status IN ('delivered','opened')) AS delivered,
		        count(*) FILTER (WHERE status = 'bounced') AS bounced,
		        count(*) FILTER (WHERE status = 'complained') AS complained,
		        count(*) FILTER (WHERE status = 'failed') AS failed
		 FROM emails
		 WHERE %s`, where,
	)

	var t port.DashboardTotals
	err := r.pool.QueryRow(ctx, query, args).Scan(&t.Sent, &t.Delivered, &t.Bounced, &t.Complained, &t.Failed)
	if err != nil {
		return nil, fmt.Errorf("querying dashboard totals: %w", err)
	}

	return &t, nil
}

func (r *DashboardRepo) GetTimeSeries(ctx context.Context, p port.DashboardStatsParams) ([]port.DashboardTimePoint, error) {
	args := pgx.NamedArgs{}
	where := scopeFilter(p, args)

	query := fmt.Sprintf(
		`SELECT date_trunc('day', created_at)::date AS date,
		        count(*) FILTER (WHERE status IN ('sent','delivered','opened')) AS sent,
		        count(*) FILTER (WHERE status IN ('delivered','opened')) AS delivered,
		        count(*) FILTER (WHERE status = 'bounced') AS bounced,
		        count(*) FILTER (WHERE status = 'failed') AS failed
		 FROM emails
		 WHERE %s
		 GROUP BY date_trunc('day', created_at)::date
		 ORDER BY 1`, where,
	)

	rows, err := r.pool.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("querying dashboard time series: %w", err)
	}

	points, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (port.DashboardTimePoint, error) {
		var pt port.DashboardTimePoint
		scanErr := row.Scan(&pt.Date, &pt.Sent, &pt.Delivered, &pt.Bounced, &pt.Failed)
		return pt, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collecting dashboard time series: %w", err)
	}

	return points, nil
}

func (r *DashboardRepo) GetRecentEmails(ctx context.Context, p port.DashboardStatsParams, limit int) ([]port.DashboardRecentEmail, error) {
	args := pgx.NamedArgs{}
	where := scopeFilter(p, args)
	args["limit"] = limit

	query := fmt.Sprintf(
		`SELECT id, tracking_id, recipient_email, template_type_slug, status, created_at
		 FROM emails
		 WHERE %s
		 ORDER BY created_at DESC, id DESC
		 LIMIT @limit`, where,
	)

	rows, err := r.pool.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("querying dashboard recent emails: %w", err)
	}

	emails, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (port.DashboardRecentEmail, error) {
		var e port.DashboardRecentEmail
		scanErr := row.Scan(&e.ID, &e.TrackingID, &e.RecipientEmail, &e.TemplateTypeSlug, &e.Status, &e.CreatedAt)
		return e, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collecting dashboard recent emails: %w", err)
	}

	return emails, nil
}

func (r *DashboardRepo) GetTotalsByAdapter(ctx context.Context, p port.DashboardStatsParams) ([]port.DashboardAdapterTotals, error) {
	args := pgx.NamedArgs{}

	// Build WHERE with table-qualified column names (JOIN makes bare names ambiguous).
	where := "e.adapter_id IS NOT NULL AND e.created_at >= @since AND e.created_at < @until"
	args["since"] = p.Since
	args["until"] = p.Until
	if p.WorkspaceID != nil {
		where += " AND e.workspace_id = @workspace_id"
		args["workspace_id"] = *p.WorkspaceID
	} else if p.TenantID != nil {
		where += " AND e.tenant_id = @tenant_id"
		args["tenant_id"] = *p.TenantID
	}

	query := fmt.Sprintf(
		`SELECT e.adapter_id,
		        COALESCE(a.name, 'unknown') AS adapter_name,
		        COALESCE(a.adapter_type::text, 'unknown') AS adapter_type,
		        e.sender_identity_id,
		        COALESCE(e.from_email, '') AS from_email,
		        count(*) FILTER (WHERE e.status IN ('sent','delivered','opened')) AS sent,
		        count(*) FILTER (WHERE e.status IN ('delivered','opened')) AS delivered,
		        count(*) FILTER (WHERE e.status = 'bounced') AS bounced,
		        count(*) FILTER (WHERE e.status = 'complained') AS complained,
		        count(*) FILTER (WHERE e.status = 'failed') AS failed
		 FROM emails e
		 LEFT JOIN adapters a ON a.id = e.adapter_id
		 WHERE %s
		 GROUP BY e.adapter_id, a.name, a.adapter_type, e.sender_identity_id, e.from_email
		 ORDER BY sent DESC`, where,
	)

	rows, err := r.pool.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("querying dashboard totals by adapter: %w", err)
	}

	results, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (port.DashboardAdapterTotals, error) {
		var at port.DashboardAdapterTotals
		scanErr := row.Scan(
			&at.AdapterID, &at.AdapterName, &at.AdapterType, &at.SenderIdentityID, &at.FromEmail,
			&at.Totals.Sent, &at.Totals.Delivered, &at.Totals.Bounced,
			&at.Totals.Complained, &at.Totals.Failed,
		)
		return at, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collecting dashboard totals by adapter: %w", err)
	}

	return results, nil
}

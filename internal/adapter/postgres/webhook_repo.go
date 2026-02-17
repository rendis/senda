package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/apperr"
)

// WebhookRepo implements port.WebhookStore using PostgreSQL.
type WebhookRepo struct {
	pool *pgxpool.Pool
}

// NewWebhookRepo creates a new WebhookRepo.
func NewWebhookRepo(pool *pgxpool.Pool) *WebhookRepo {
	return &WebhookRepo{pool: pool}
}

func (r *WebhookRepo) Create(ctx context.Context, wh *domain.Webhook) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO webhooks (id, workspace_id, url, secret, events, is_active)
		 VALUES (@id, @workspace_id, @url, @secret, @events, @is_active)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":           wh.ID,
			"workspace_id": wh.WorkspaceID,
			"url":          wh.URL,
			"secret":       wh.Secret,
			"events":       wh.Events,
			"is_active":    wh.IsActive,
		},
	)

	if err := row.Scan(&wh.CreatedAt, &wh.UpdatedAt); err != nil {
		return fmt.Errorf("inserting webhook: %w", err)
	}

	return nil
}

func (r *WebhookRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, url, secret, events, is_active,
		        consecutive_failures, last_failure_at, disabled_at, created_at, updated_at
		 FROM webhooks
		 WHERE id = @id`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return nil, fmt.Errorf("querying webhook: %w", err)
	}

	wh, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[domain.Webhook])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("webhook %s not found", id)
		}
		return nil, fmt.Errorf("scanning webhook: %w", err)
	}

	return wh, nil
}

func (r *WebhookRepo) Update(ctx context.Context, wh *domain.Webhook) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE webhooks
		 SET url = @url, secret = @secret, events = @events, is_active = @is_active,
		     consecutive_failures = @consecutive_failures, last_failure_at = @last_failure_at,
		     disabled_at = @disabled_at, updated_at = now()
		 WHERE id = @id
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":                   wh.ID,
			"url":                  wh.URL,
			"secret":               wh.Secret,
			"events":               wh.Events,
			"is_active":            wh.IsActive,
			"consecutive_failures": wh.ConsecutiveFailures,
			"last_failure_at":      wh.LastFailureAt,
			"disabled_at":          wh.DisabledAt,
		},
	)

	if err := row.Scan(&wh.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("webhook %s not found", wh.ID)
		}
		return fmt.Errorf("updating webhook: %w", err)
	}

	return nil
}

func (r *WebhookRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM webhooks WHERE id = @id`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("webhook %s not found", id)
	}
	return nil
}

func (r *WebhookRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Webhook], error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, err
	}

	fetchLimit := limit + 1

	var rows pgx.Rows
	if afterID != nil {
		rows, err = r.pool.Query(ctx,
			`SELECT id, workspace_id, url, secret, events, is_active,
			        consecutive_failures, last_failure_at, disabled_at, created_at, updated_at
			 FROM webhooks
			 WHERE workspace_id = @workspace_id AND id < @after_id
			 ORDER BY id DESC
			 LIMIT @limit`,
			pgx.NamedArgs{"workspace_id": workspaceID, "after_id": *afterID, "limit": fetchLimit},
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, workspace_id, url, secret, events, is_active,
			        consecutive_failures, last_failure_at, disabled_at, created_at, updated_at
			 FROM webhooks
			 WHERE workspace_id = @workspace_id
			 ORDER BY id DESC
			 LIMIT @limit`,
			pgx.NamedArgs{"workspace_id": workspaceID, "limit": fetchLimit},
		)
	}
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}

	webhooks, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Webhook])
	if err != nil {
		return nil, fmt.Errorf("collecting webhooks: %w", err)
	}

	result := &port.PageResult[domain.Webhook]{}
	if len(webhooks) > limit {
		webhooks = webhooks[:limit]
		result.HasMore = true
		result.NextCursor = EncodeCursor(webhooks[limit-1].ID)
	}
	result.Items = webhooks

	return result, nil
}

func (r *WebhookRepo) GetActiveByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Webhook, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, url, secret, events, is_active,
		        consecutive_failures, last_failure_at, disabled_at, created_at, updated_at
		 FROM webhooks
		 WHERE workspace_id = @workspace_id AND is_active = true AND disabled_at IS NULL`,
		pgx.NamedArgs{"workspace_id": workspaceID},
	)
	if err != nil {
		return nil, fmt.Errorf("querying active webhooks: %w", err)
	}

	webhooks, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Webhook])
	if err != nil {
		return nil, fmt.Errorf("collecting active webhooks: %w", err)
	}

	return webhooks, nil
}

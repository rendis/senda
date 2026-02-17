package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/apperr"
)

// EmailRepo implements port.EmailStore using PostgreSQL.
type EmailRepo struct {
	pool *pgxpool.Pool
}

// NewEmailRepo creates a new EmailRepo.
func NewEmailRepo(pool *pgxpool.Pool) *EmailRepo {
	return &EmailRepo{pool: pool}
}

func (r *EmailRepo) Create(ctx context.Context, email *domain.Email) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO emails (
			id, tracking_id, external_id, workspace_id, tenant_id,
			template_id, template_version_id, template_type_slug, template_ref,
			recipient_email, cc, bcc, from_email, from_name, reply_to,
			subject_rendered, body_mjml, locale, status, adapter_id,
			provider_message_id, variables_snapshot, injectors_snapshot,
			retry_count, max_retries, next_retry_at
		) VALUES (
			@id, @tracking_id, @external_id, @workspace_id, @tenant_id,
			@template_id, @template_version_id, @template_type_slug, @template_ref,
			@recipient_email, @cc, @bcc, @from_email, @from_name, @reply_to,
			@subject_rendered, @body_mjml, @locale, @status, @adapter_id,
			@provider_message_id, @variables_snapshot, @injectors_snapshot,
			@retry_count, @max_retries, @next_retry_at
		) RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":                  email.ID,
			"tracking_id":        email.TrackingID,
			"external_id":        email.ExternalID,
			"workspace_id":       email.WorkspaceID,
			"tenant_id":          email.TenantID,
			"template_id":        email.TemplateID,
			"template_version_id": email.TemplateVersionID,
			"template_type_slug": email.TemplateTypeSlug,
			"template_ref":       email.TemplateRef,
			"recipient_email":    email.RecipientEmail,
			"cc":                 email.CC,
			"bcc":                email.BCC,
			"from_email":         email.FromEmail,
			"from_name":          email.FromName,
			"reply_to":           email.ReplyTo,
			"subject_rendered":   email.SubjectRendered,
			"body_mjml":          email.BodyMJML,
			"locale":             email.Locale,
			"status":             email.Status,
			"adapter_id":         email.AdapterID,
			"provider_message_id": email.ProviderMessageID,
			"variables_snapshot": email.VariablesSnapshot,
			"injectors_snapshot": email.InjectorsSnapshot,
			"retry_count":        email.RetryCount,
			"max_retries":        email.MaxRetries,
			"next_retry_at":      email.NextRetryAt,
		},
	)

	if err := row.Scan(&email.CreatedAt, &email.UpdatedAt); err != nil {
		return fmt.Errorf("inserting email: %w", err)
	}

	return nil
}

func (r *EmailRepo) GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tracking_id, external_id, workspace_id, tenant_id,
		        template_id, template_version_id, template_type_slug, template_ref,
		        recipient_email, cc, bcc, from_email, from_name, reply_to,
		        subject_rendered, body_mjml, locale, status, adapter_id,
		        provider_message_id, variables_snapshot, injectors_snapshot,
		        retry_count, max_retries, next_retry_at, created_at, updated_at
		 FROM emails
		 WHERE tracking_id = @tracking_id`,
		pgx.NamedArgs{"tracking_id": trackingID},
	)

	return scanEmail(row)
}

func (r *EmailRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.EmailStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE emails SET status = @status, updated_at = now() WHERE id = @id`,
		pgx.NamedArgs{"id": id, "status": status},
	)
	if err != nil {
		return fmt.Errorf("updating email status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("email %s not found", id)
	}
	return nil
}

func (r *EmailRepo) UpdateRetry(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE emails SET retry_count = @retry_count, next_retry_at = @next_retry_at, updated_at = now()
		 WHERE id = @id`,
		pgx.NamedArgs{"id": id, "retry_count": retryCount, "next_retry_at": nextRetryAt},
	)
	if err != nil {
		return fmt.Errorf("updating email retry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("email %s not found", id)
	}
	return nil
}

func (r *EmailRepo) AddEvent(ctx context.Context, event *domain.EmailEvent) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO email_events (id, email_id, event_type, occurred_at, metadata)
		 VALUES (@id, @email_id, @event_type, @occurred_at, @metadata)
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":          event.ID,
			"email_id":    event.EmailID,
			"event_type":  event.EventType,
			"occurred_at": event.OccurredAt,
			"metadata":    event.Metadata,
		},
	)

	if err := row.Scan(&event.CreatedAt); err != nil {
		return fmt.Errorf("inserting email event: %w", err)
	}

	return nil
}

func (r *EmailRepo) GetEvents(ctx context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email_id, event_type, occurred_at, metadata, created_at
		 FROM email_events
		 WHERE email_id = @email_id
		 ORDER BY occurred_at`,
		pgx.NamedArgs{"email_id": emailID},
	)
	if err != nil {
		return nil, fmt.Errorf("querying email events: %w", err)
	}

	events, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.EmailEvent])
	if err != nil {
		return nil, fmt.Errorf("collecting email events: %w", err)
	}

	return events, nil
}

func (r *EmailRepo) QueryByExternalID(ctx context.Context, wsID uuid.UUID, externalID string, cursor string, limit int) ([]*domain.Email, string, error) {
	return r.queryEmails(ctx, cursor, limit,
		`workspace_id = @workspace_id AND external_id = @external_id`,
		pgx.NamedArgs{"workspace_id": wsID, "external_id": externalID},
	)
}

func (r *EmailRepo) QueryByRecipient(ctx context.Context, wsID uuid.UUID, email string, cursor string, limit int) ([]*domain.Email, string, error) {
	return r.queryEmails(ctx, cursor, limit,
		`workspace_id = @workspace_id AND recipient_email = @recipient_email`,
		pgx.NamedArgs{"workspace_id": wsID, "recipient_email": email},
	)
}

func (r *EmailRepo) QueryByWorkspace(ctx context.Context, wsID uuid.UUID, filters port.EmailFilters, cursor string, limit int) ([]*domain.Email, string, error) {
	where := `workspace_id = @workspace_id`
	args := pgx.NamedArgs{"workspace_id": wsID}

	if filters.Status != nil {
		where += ` AND status = @status`
		args["status"] = *filters.Status
	}
	if filters.TemplateTypeSlug != nil {
		where += ` AND template_type_slug = @template_type_slug`
		args["template_type_slug"] = *filters.TemplateTypeSlug
	}
	if filters.Since != nil {
		where += ` AND created_at >= @since`
		args["since"] = *filters.Since
	}
	if filters.Until != nil {
		where += ` AND created_at < @until`
		args["until"] = *filters.Until
	}

	return r.queryEmails(ctx, cursor, limit, where, args)
}

func (r *EmailRepo) QueryByExternalIDGlobal(ctx context.Context, externalID string, cursor string, limit int) ([]*domain.Email, string, error) {
	return r.queryEmails(ctx, cursor, limit,
		`external_id = @external_id`,
		pgx.NamedArgs{"external_id": externalID},
	)
}

// queryEmails is the shared query helper for all email queries.
// It uses composite (created_at, id) cursors for partitioned tables.
func (r *EmailRepo) queryEmails(ctx context.Context, cursor string, limit int, where string, args pgx.NamedArgs) ([]*domain.Email, string, error) {
	limit = NormalizeLimit(limit)
	fetchLimit := limit + 1

	if cursor != "" {
		cursorTime, cursorID, err := DecodeTimeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		where += ` AND (created_at, id) < (@cursor_time, @cursor_id)`
		args["cursor_time"] = cursorTime
		args["cursor_id"] = cursorID
	}

	args["limit"] = fetchLimit

	query := fmt.Sprintf(
		`SELECT id, tracking_id, external_id, workspace_id, tenant_id,
		        template_id, template_version_id, template_type_slug, template_ref,
		        recipient_email, cc, bcc, from_email, from_name, reply_to,
		        subject_rendered, body_mjml, locale, status, adapter_id,
		        provider_message_id, variables_snapshot, injectors_snapshot,
		        retry_count, max_retries, next_retry_at, created_at, updated_at
		 FROM emails
		 WHERE %s
		 ORDER BY created_at DESC, id DESC
		 LIMIT @limit`, where,
	)

	rows, err := r.pool.Query(ctx, query, args)
	if err != nil {
		return nil, "", fmt.Errorf("querying emails: %w", err)
	}

	emails, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Email, error) {
		return collectEmail(row)
	})
	if err != nil {
		return nil, "", fmt.Errorf("collecting emails: %w", err)
	}

	var nextCursor string
	if len(emails) > limit {
		emails = emails[:limit]
		last := emails[limit-1]
		nextCursor = EncodeTimeCursor(last.CreatedAt, last.ID)
	}

	return emails, nextCursor, nil
}

func scanEmail(row pgx.Row) (*domain.Email, error) {
	var e domain.Email
	err := row.Scan(
		&e.ID, &e.TrackingID, &e.ExternalID, &e.WorkspaceID, &e.TenantID,
		&e.TemplateID, &e.TemplateVersionID, &e.TemplateTypeSlug, &e.TemplateRef,
		&e.RecipientEmail, &e.CC, &e.BCC, &e.FromEmail, &e.FromName, &e.ReplyTo,
		&e.SubjectRendered, &e.BodyMJML, &e.Locale, &e.Status, &e.AdapterID,
		&e.ProviderMessageID, &e.VariablesSnapshot, &e.InjectorsSnapshot,
		&e.RetryCount, &e.MaxRetries, &e.NextRetryAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("email not found")
		}
		return nil, fmt.Errorf("scanning email: %w", err)
	}
	return &e, nil
}

func collectEmail(row pgx.CollectableRow) (*domain.Email, error) {
	var e domain.Email
	err := row.Scan(
		&e.ID, &e.TrackingID, &e.ExternalID, &e.WorkspaceID, &e.TenantID,
		&e.TemplateID, &e.TemplateVersionID, &e.TemplateTypeSlug, &e.TemplateRef,
		&e.RecipientEmail, &e.CC, &e.BCC, &e.FromEmail, &e.FromName, &e.ReplyTo,
		&e.SubjectRendered, &e.BodyMJML, &e.Locale, &e.Status, &e.AdapterID,
		&e.ProviderMessageID, &e.VariablesSnapshot, &e.InjectorsSnapshot,
		&e.RetryCount, &e.MaxRetries, &e.NextRetryAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}


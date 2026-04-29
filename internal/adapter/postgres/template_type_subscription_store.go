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

// Compile-time interface check.
var _ port.TemplateTypeSubscriptionStore = (*TemplateTypeSubscriptionStore)(nil)

// TemplateTypeSubscriptionStore is the Postgres adapter for template_type_subscriptions.
type TemplateTypeSubscriptionStore struct {
	db *pgxpool.Pool
}

// NewTemplateTypeSubscriptionStore returns a new TemplateTypeSubscriptionStore backed by pool.
func NewTemplateTypeSubscriptionStore(db *pgxpool.Pool) *TemplateTypeSubscriptionStore {
	return &TemplateTypeSubscriptionStore{db: db}
}

// Upsert inserts or updates the subscription state for (workspace, template_type, email).
func (s *TemplateTypeSubscriptionStore) Upsert(ctx context.Context, sub *domain.TemplateTypeSubscription) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO template_type_subscriptions
			(id, workspace_id, template_type_id, email, subscribed, source, source_email_id, actor_id, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (workspace_id, template_type_id, email) DO UPDATE
			SET subscribed      = EXCLUDED.subscribed,
			    source          = EXCLUDED.source,
			    source_email_id = EXCLUDED.source_email_id,
			    actor_id        = EXCLUDED.actor_id,
			    notes           = EXCLUDED.notes,
			    updated_at      = now()
	`,
		sub.ID, sub.WorkspaceID, sub.TemplateTypeID, sub.Email,
		sub.Subscribed, sub.Source, sub.SourceEmailID, sub.ActorID, sub.Notes,
	)
	if err != nil {
		return fmt.Errorf("tts: upsert: %w", err)
	}
	return nil
}

// GetState returns the subscription row for (workspaceID, templateTypeID, email),
// or a 404 AppError if none exists.
func (s *TemplateTypeSubscriptionStore) GetState(
	ctx context.Context, workspaceID, templateTypeID uuid.UUID, email string,
) (*domain.TemplateTypeSubscription, error) {
	var sub domain.TemplateTypeSubscription
	err := s.db.QueryRow(ctx, `
		SELECT id, workspace_id, template_type_id, email, subscribed, source,
		       source_email_id, actor_id, notes, created_at, updated_at
		FROM template_type_subscriptions
		WHERE workspace_id = $1 AND template_type_id = $2 AND email = $3
	`, workspaceID, templateTypeID, email).Scan(
		&sub.ID, &sub.WorkspaceID, &sub.TemplateTypeID, &sub.Email,
		&sub.Subscribed, &sub.Source,
		&sub.SourceEmailID, &sub.ActorID, &sub.Notes,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("template_type_subscription not found for (ws=%s, tt=%s, email=%s)", workspaceID, templateTypeID, email)
	}
	if err != nil {
		return nil, fmt.Errorf("tts: get state: %w", err)
	}
	return &sub, nil
}

// ListOptOutsForRecipient returns all subscription rows for (workspaceID, email).
func (s *TemplateTypeSubscriptionStore) ListOptOutsForRecipient(
	ctx context.Context, workspaceID uuid.UUID, email string,
) ([]*domain.TemplateTypeSubscription, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, workspace_id, template_type_id, email, subscribed, source,
		       source_email_id, actor_id, notes, created_at, updated_at
		FROM template_type_subscriptions
		WHERE workspace_id = $1 AND email = $2
	`, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("tts: list: %w", err)
	}
	defer rows.Close()

	var out []*domain.TemplateTypeSubscription
	for rows.Next() {
		var sub domain.TemplateTypeSubscription
		if err := rows.Scan(
			&sub.ID, &sub.WorkspaceID, &sub.TemplateTypeID, &sub.Email,
			&sub.Subscribed, &sub.Source,
			&sub.SourceEmailID, &sub.ActorID, &sub.Notes,
			&sub.CreatedAt, &sub.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("tts: scan: %w", err)
		}
		out = append(out, &sub)
	}
	return out, rows.Err()
}

// BatchCheckOptOut returns the subset of emails that have an explicit opt-out row
// (subscribed = false) for the given (workspaceID, templateTypeID). Emails with no
// row or subscribed=true are not included. An empty input returns an empty map
// immediately without querying the DB.
func (s *TemplateTypeSubscriptionStore) BatchCheckOptOut(
	ctx context.Context, workspaceID, templateTypeID uuid.UUID, emails []string,
) (map[string]struct{}, error) {
	if len(emails) == 0 {
		return map[string]struct{}{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT email
		FROM template_type_subscriptions
		WHERE workspace_id    = $1
		  AND template_type_id = $2
		  AND email            = ANY($3)
		  AND subscribed       = false
	`, workspaceID, templateTypeID, emails)
	if err != nil {
		return nil, fmt.Errorf("tts: batch check: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("tts: scan email: %w", err)
		}
		out[e] = struct{}{}
	}
	return out, rows.Err()
}

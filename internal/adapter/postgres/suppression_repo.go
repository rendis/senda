package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

// SuppressionRepo implements port.SuppressionStore using PostgreSQL.
type SuppressionRepo struct {
	pool *pgxpool.Pool
}

// NewSuppressionRepo creates a new SuppressionRepo.
func NewSuppressionRepo(pool *pgxpool.Pool) *SuppressionRepo {
	return &SuppressionRepo{pool: pool}
}

func (r *SuppressionRepo) AddGlobal(ctx context.Context, entry *domain.SuppressionGlobal) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO suppression_global (id, email, reason, source_email_id, notes)
		 VALUES (@id, @email, @reason, @source_email_id, @notes)
		 ON CONFLICT (email) DO UPDATE SET
			reason = @reason,
			source_email_id = @source_email_id,
			notes = @notes,
			removed_at = NULL,
			removed_by = NULL,
			removal_reason = NULL
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":              entry.ID,
			"email":           entry.Email,
			"reason":          entry.Reason,
			"source_email_id": entry.SourceEmailID,
			"notes":           entry.Notes,
		},
	)

	if err := row.Scan(&entry.CreatedAt); err != nil {
		return fmt.Errorf("upserting global suppression: %w", err)
	}

	return nil
}

func (r *SuppressionRepo) IsGloballySuppressed(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM suppression_global WHERE email = @email AND removed_at IS NULL)`,
		pgx.NamedArgs{"email": email},
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking global suppression: %w", err)
	}
	return exists, nil
}

func (r *SuppressionRepo) RemoveGlobal(ctx context.Context, email string, removedBy uuid.UUID, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE suppression_global
		 SET removed_at = now(), removed_by = @removed_by, removal_reason = @reason
		 WHERE email = @email AND removed_at IS NULL`,
		pgx.NamedArgs{"email": email, "removed_by": removedBy, "reason": reason},
	)
	if err != nil {
		return fmt.Errorf("removing global suppression: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("global suppression for %q not found", email)
	}
	return nil
}

func (r *SuppressionRepo) AddWorkspace(ctx context.Context, entry *domain.SuppressionWorkspace) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO suppression_workspace (id, workspace_id, email, reason, source_email_id, notes)
		 VALUES (@id, @workspace_id, @email, @reason, @source_email_id, @notes)
		 ON CONFLICT (workspace_id, email) DO UPDATE SET
			reason = @reason,
			source_email_id = @source_email_id,
			notes = @notes,
			removed_at = NULL,
			removed_by = NULL,
			removal_reason = NULL
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":              entry.ID,
			"workspace_id":    entry.WorkspaceID,
			"email":           entry.Email,
			"reason":          entry.Reason,
			"source_email_id": entry.SourceEmailID,
			"notes":           entry.Notes,
		},
	)

	if err := row.Scan(&entry.CreatedAt); err != nil {
		return fmt.Errorf("upserting workspace suppression: %w", err)
	}

	return nil
}

func (r *SuppressionRepo) IsWorkspaceSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM suppression_workspace
		 WHERE workspace_id = @workspace_id AND email = @email AND removed_at IS NULL)`,
		pgx.NamedArgs{"workspace_id": wsID, "email": email},
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking workspace suppression: %w", err)
	}
	return exists, nil
}

func (r *SuppressionRepo) IsSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error) {
	var reason string
	err := r.pool.QueryRow(ctx,
		`SELECT reason FROM (
			SELECT reason FROM suppression_global WHERE email = @email AND removed_at IS NULL
			UNION ALL
			SELECT reason FROM suppression_workspace WHERE workspace_id = @workspace_id AND email = @email AND removed_at IS NULL
			LIMIT 1
		 ) AS s`,
		pgx.NamedArgs{"email": email, "workspace_id": wsID},
	).Scan(&reason)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("checking combined suppression: %w", err)
	}
	return true, reason, nil
}

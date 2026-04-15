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

func (r *SuppressionRepo) GetSuppressionStatuses(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]port.SuppressionStatus, error) {
	if len(emails) == 0 {
		return map[string]port.SuppressionStatus{}, nil
	}

	rows, err := r.pool.Query(ctx,
		`WITH requested AS (
			SELECT DISTINCT email
			FROM unnest(@emails::text[]) AS email
		)
		SELECT
			requested.email,
			(global_match.email IS NOT NULL OR workspace_match.email IS NOT NULL) AS suppressed,
			COALESCE(global_match.reason::text, workspace_match.reason::text, '') AS reason
		FROM requested
		LEFT JOIN suppression_global AS global_match
			ON global_match.email = requested.email AND global_match.removed_at IS NULL
		LEFT JOIN suppression_workspace AS workspace_match
			ON workspace_match.workspace_id = @workspace_id
			AND workspace_match.email = requested.email
			AND workspace_match.removed_at IS NULL`,
		pgx.NamedArgs{
			"workspace_id": wsID,
			"emails":       emails,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("checking suppression set: %w", err)
	}
	defer rows.Close()

	statuses := make(map[string]port.SuppressionStatus, len(emails))
	for rows.Next() {
		var email string
		var suppressed bool
		var reason string
		if err := rows.Scan(&email, &suppressed, &reason); err != nil {
			return nil, fmt.Errorf("scan suppression set row: %w", err)
		}
		statuses[email] = port.SuppressionStatus{
			Suppressed: suppressed,
			Reason:     reason,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate suppression set rows: %w", err)
	}

	return statuses, nil
}

func (r *SuppressionRepo) CheckBatch(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]string, error) {
	statuses, err := r.GetSuppressionStatuses(ctx, wsID, emails)
	if err != nil {
		return nil, err
	}

	suppressed := make(map[string]string, len(statuses))
	for email, status := range statuses {
		if status.Suppressed {
			suppressed[email] = status.Reason
		}
	}
	return suppressed, nil
}

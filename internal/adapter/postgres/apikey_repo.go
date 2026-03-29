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

// APIKeyRepo implements port.APIKeyStore using PostgreSQL.
type APIKeyRepo struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepo creates a new APIKeyRepo.
func NewAPIKeyRepo(pool *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{pool: pool}
}

func (r *APIKeyRepo) Create(ctx context.Context, key *domain.APIKey) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO api_keys (id, workspace_id, name, key_hash, key_prefix, key_hint, created_by)
		 VALUES (@id, @workspace_id, @name, @key_hash, @key_prefix, @key_hint, @created_by)
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":           key.ID,
			"workspace_id": key.WorkspaceID,
			"name":         key.Name,
			"key_hash":     key.KeyHash,
			"key_prefix":   key.KeyPrefix,
			"key_hint":     key.KeyHint,
			"created_by":   key.CreatedBy,
		},
	)

	if err := row.Scan(&key.CreatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting api key: %w", err)
	}

	return nil
}

func (r *APIKeyRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, key_hash, key_prefix, key_hint, created_by,
		        last_used_at, revoked_at, created_at
		 FROM api_keys
		 WHERE id = @id AND revoked_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanAPIKey(row)
}

func (r *APIKeyRepo) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, key_hash, key_prefix, key_hint, created_by,
		        last_used_at, revoked_at, created_at
		 FROM api_keys
		 WHERE key_hash = @key_hash AND revoked_at IS NULL`,
		pgx.NamedArgs{"key_hash": hash},
	)

	return scanAPIKey(row)
}

func (r *APIKeyRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = now() WHERE id = @id AND revoked_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("revoking api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("api key %s not found or already revoked", id)
	}
	return nil
}

func (r *APIKeyRepo) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = now() WHERE id = @id`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("touching api key last_used_at: %w", err)
	}
	return nil
}

func (r *APIKeyRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.APIKey], error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, err
	}

	fetchLimit := limit + 1

	var rows pgx.Rows
	if afterID != nil {
		rows, err = r.pool.Query(ctx,
			`SELECT id, workspace_id, name, key_prefix, key_hint, created_by,
			        last_used_at, revoked_at, created_at
			 FROM api_keys
			 WHERE workspace_id = @workspace_id AND id < @after_id
			 ORDER BY id DESC
			 LIMIT @limit`,
			pgx.NamedArgs{"workspace_id": workspaceID, "after_id": *afterID, "limit": fetchLimit},
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, workspace_id, name, key_prefix, key_hint, created_by,
			        last_used_at, revoked_at, created_at
			 FROM api_keys
			 WHERE workspace_id = @workspace_id
			 ORDER BY id DESC
			 LIMIT @limit`,
			pgx.NamedArgs{"workspace_id": workspaceID, "limit": fetchLimit},
		)
	}
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}

	// Manual scan because we exclude key_hash from the SELECT.
	keys, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.APIKey, error) {
		var k domain.APIKey
		err := row.Scan(
			&k.ID, &k.WorkspaceID, &k.Name, &k.KeyPrefix, &k.KeyHint, &k.CreatedBy,
			&k.LastUsedAt, &k.RevokedAt, &k.CreatedAt,
		)
		return &k, err
	})
	if err != nil {
		return nil, fmt.Errorf("collecting api keys: %w", err)
	}

	result := &port.PageResult[domain.APIKey]{}
	if len(keys) > limit {
		keys = keys[:limit]
		result.HasMore = true
		result.NextCursor = EncodeCursor(keys[limit-1].ID)
	}
	result.Items = keys

	return result, nil
}

func scanAPIKey(row pgx.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	err := row.Scan(
		&k.ID, &k.WorkspaceID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.KeyHint, &k.CreatedBy,
		&k.LastUsedAt, &k.RevokedAt, &k.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("api key not found")
		}
		return nil, fmt.Errorf("scanning api key: %w", err)
	}
	return &k, nil
}

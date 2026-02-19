package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/pkg/apperr"
)

// AdapterIdentityRepo implements port.AdapterIdentityStore using PostgreSQL.
type AdapterIdentityRepo struct {
	pool *pgxpool.Pool
}

func NewAdapterIdentityRepo(pool *pgxpool.Pool) *AdapterIdentityRepo {
	return &AdapterIdentityRepo{pool: pool}
}

func (r *AdapterIdentityRepo) Create(ctx context.Context, identity *domain.AdapterIdentity) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO adapter_identities (id, adapter_id, identity, identity_type, status, sending_enabled, is_default, display_name, source, last_synced_at)
		 VALUES (@id, @adapter_id, @identity, @identity_type, @status, @sending_enabled, @is_default, @display_name, @source, @last_synced_at)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":              identity.ID,
			"adapter_id":      identity.AdapterID,
			"identity":        identity.Identity,
			"identity_type":   identity.IdentityType,
			"status":          identity.Status,
			"sending_enabled": identity.SendingEnabled,
			"is_default":      identity.IsDefault,
			"display_name":    identity.DisplayName,
			"source":          identity.Source,
			"last_synced_at":  identity.LastSyncedAt,
		},
	)

	if err := row.Scan(&identity.CreatedAt, &identity.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting adapter identity: %w", err)
	}
	return nil
}

func (r *AdapterIdentityRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, adapter_id, identity, identity_type, status, sending_enabled, is_default,
		        display_name, source, last_synced_at, created_at, updated_at
		 FROM adapter_identities
		 WHERE id = @id`,
		pgx.NamedArgs{"id": id},
	)
	return scanAdapterIdentity(row)
}

func (r *AdapterIdentityRepo) Update(ctx context.Context, identity *domain.AdapterIdentity) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE adapter_identities
		 SET status = @status,
		     sending_enabled = @sending_enabled,
		     is_default = @is_default,
		     display_name = @display_name,
		     last_synced_at = @last_synced_at,
		     updated_at = now()
		 WHERE id = @id
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":              identity.ID,
			"status":          identity.Status,
			"sending_enabled": identity.SendingEnabled,
			"is_default":      identity.IsDefault,
			"display_name":    identity.DisplayName,
			"last_synced_at":  identity.LastSyncedAt,
		},
	)

	if err := row.Scan(&identity.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("adapter identity %s not found", identity.ID)
		}
		return fmt.Errorf("updating adapter identity: %w", err)
	}
	return nil
}

func (r *AdapterIdentityRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM adapter_identities WHERE id = @id`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("deleting adapter identity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("adapter identity %s not found", id)
	}
	return nil
}

func (r *AdapterIdentityRepo) ListByAdapter(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, adapter_id, identity, identity_type, status, sending_enabled, is_default,
		        display_name, source, last_synced_at, created_at, updated_at
		 FROM adapter_identities
		 WHERE adapter_id = @adapter_id
		 ORDER BY is_default DESC, identity_type ASC, identity ASC`,
		pgx.NamedArgs{"adapter_id": adapterID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing adapter identities: %w", err)
	}

	identities, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.AdapterIdentity])
	if err != nil {
		return nil, fmt.Errorf("collecting adapter identities: %w", err)
	}
	return identities, nil
}

func (r *AdapterIdentityRepo) GetDefault(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, adapter_id, identity, identity_type, status, sending_enabled, is_default,
		        display_name, source, last_synced_at, created_at, updated_at
		 FROM adapter_identities
		 WHERE adapter_id = @adapter_id AND is_default = true`,
		pgx.NamedArgs{"adapter_id": adapterID},
	)
	return scanAdapterIdentity(row)
}

func (r *AdapterIdentityRepo) SetDefault(ctx context.Context, adapterID uuid.UUID, identityID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Unset previous default
	_, err = tx.Exec(ctx,
		`UPDATE adapter_identities SET is_default = false, updated_at = now()
		 WHERE adapter_id = @adapter_id AND is_default = true`,
		pgx.NamedArgs{"adapter_id": adapterID},
	)
	if err != nil {
		return fmt.Errorf("unsetting previous default: %w", err)
	}

	// Set new default
	tag, err := tx.Exec(ctx,
		`UPDATE adapter_identities SET is_default = true, updated_at = now()
		 WHERE id = @id AND adapter_id = @adapter_id`,
		pgx.NamedArgs{"id": identityID, "adapter_id": adapterID},
	)
	if err != nil {
		return fmt.Errorf("setting new default: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("adapter identity %s not found", identityID)
	}

	return tx.Commit(ctx)
}

func (r *AdapterIdentityRepo) UpsertBatch(ctx context.Context, adapterID uuid.UUID, identities []*domain.AdapterIdentity) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, identity := range identities {
		_, err = tx.Exec(ctx,
			`INSERT INTO adapter_identities (id, adapter_id, identity, identity_type, status, sending_enabled, source, last_synced_at)
			 VALUES (@id, @adapter_id, @identity, @identity_type, @status, @sending_enabled, @source, now())
			 ON CONFLICT (adapter_id, identity) DO UPDATE SET
			     status = EXCLUDED.status,
			     sending_enabled = EXCLUDED.sending_enabled,
			     last_synced_at = now(),
			     updated_at = now()`,
			pgx.NamedArgs{
				"id":              identity.ID,
				"adapter_id":      adapterID,
				"identity":        identity.Identity,
				"identity_type":   identity.IdentityType,
				"status":          identity.Status,
				"sending_enabled": identity.SendingEnabled,
				"source":          identity.Source,
			},
		)
		if err != nil {
			return fmt.Errorf("upserting identity %s: %w", identity.Identity, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *AdapterIdentityRepo) DeleteStale(ctx context.Context, adapterID uuid.UUID, keepIdentities []string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM adapter_identities
		 WHERE adapter_id = @adapter_id
		   AND source = 'provider'
		   AND identity != ALL(@keep)`,
		pgx.NamedArgs{
			"adapter_id": adapterID,
			"keep":       keepIdentities,
		},
	)
	if err != nil {
		return fmt.Errorf("deleting stale identities: %w", err)
	}
	return nil
}

func scanAdapterIdentity(row pgx.Row) (*domain.AdapterIdentity, error) {
	var ai domain.AdapterIdentity
	err := row.Scan(
		&ai.ID, &ai.AdapterID, &ai.Identity, &ai.IdentityType, &ai.Status,
		&ai.SendingEnabled, &ai.IsDefault, &ai.DisplayName, &ai.Source,
		&ai.LastSyncedAt, &ai.CreatedAt, &ai.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("adapter identity not found")
		}
		return nil, fmt.Errorf("scanning adapter identity: %w", err)
	}
	return &ai, nil
}

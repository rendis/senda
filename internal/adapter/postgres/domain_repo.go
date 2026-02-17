package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/apperr"
)

// DomainRepo implements port.DomainStore using PostgreSQL.
type DomainRepo struct {
	pool *pgxpool.Pool
}

// NewDomainRepo creates a new DomainRepo.
func NewDomainRepo(pool *pgxpool.Pool) *DomainRepo {
	return &DomainRepo{pool: pool}
}

func (r *DomainRepo) Create(ctx context.Context, d *domain.Domain) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO domains (id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
		                      dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error)
		 VALUES (@id, @workspace_id, @domain_name, @status, @dkim_selector, @dkim_private_key_encrypted,
		         @dkim_public_key, @dns_records, @verified_at, @last_check_at, @next_check_at, @last_error)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":                         d.ID,
			"workspace_id":               d.WorkspaceID,
			"domain_name":                d.DomainName,
			"status":                     d.Status,
			"dkim_selector":              d.DKIMSelector,
			"dkim_private_key_encrypted": d.DKIMPrivateKeyEncrypted,
			"dkim_public_key":            d.DKIMPublicKey,
			"dns_records":                d.DNSRecords,
			"verified_at":                d.VerifiedAt,
			"last_check_at":              d.LastCheckAt,
			"next_check_at":              d.NextCheckAt,
			"last_error":                 d.LastError,
		},
	)

	if err := row.Scan(&d.CreatedAt, &d.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperr.Conflict("domain %q already exists in this scope", d.DomainName)
		}
		return fmt.Errorf("inserting domain: %w", err)
	}

	return nil
}

func (r *DomainRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Domain, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
		        dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error,
		        created_at, updated_at, deleted_at
		 FROM domains
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanDomain(row)
}

func (r *DomainRepo) Update(ctx context.Context, d *domain.Domain) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE domains
		 SET domain_name = @domain_name,
		     status = @status,
		     dkim_selector = @dkim_selector,
		     dkim_private_key_encrypted = @dkim_private_key_encrypted,
		     dkim_public_key = @dkim_public_key,
		     dns_records = @dns_records,
		     verified_at = @verified_at,
		     last_check_at = @last_check_at,
		     next_check_at = @next_check_at,
		     last_error = @last_error,
		     updated_at = now()
		 WHERE id = @id AND deleted_at IS NULL
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":                         d.ID,
			"domain_name":                d.DomainName,
			"status":                     d.Status,
			"dkim_selector":              d.DKIMSelector,
			"dkim_private_key_encrypted": d.DKIMPrivateKeyEncrypted,
			"dkim_public_key":            d.DKIMPublicKey,
			"dns_records":                d.DNSRecords,
			"verified_at":                d.VerifiedAt,
			"last_check_at":              d.LastCheckAt,
			"next_check_at":              d.NextCheckAt,
			"last_error":                 d.LastError,
		},
	)

	if err := row.Scan(&d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("domain %s not found", d.ID)
		}
		return fmt.Errorf("updating domain: %w", err)
	}

	return nil
}

func (r *DomainRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE domains SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting domain: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("domain %s not found", id)
	}
	return nil
}

func (r *DomainRepo) ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error) {
	nonNull, includeGlobal := splitChain(scopes)

	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
		        dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error,
		        created_at, updated_at, deleted_at
		 FROM domains
		 WHERE (workspace_id = ANY(@scopes) OR (@include_global::bool AND workspace_id IS NULL))
		   AND deleted_at IS NULL
		 ORDER BY id DESC`,
		pgx.NamedArgs{
			"scopes":         nonNull,
			"include_global": includeGlobal,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("listing domains in chain: %w", err)
	}

	domains, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Domain])
	if err != nil {
		return nil, fmt.Errorf("collecting domains: %w", err)
	}

	return domains, nil
}

func (r *DomainRepo) ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Domain], error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, err
	}

	fetchLimit := limit + 1

	var rows pgx.Rows
	if workspaceID == nil {
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
				        dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error,
				        created_at, updated_at, deleted_at
				 FROM domains
				 WHERE workspace_id IS NULL AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
				        dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error,
				        created_at, updated_at, deleted_at
				 FROM domains
				 WHERE workspace_id IS NULL AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"limit": fetchLimit},
			)
		}
	} else {
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
				        dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error,
				        created_at, updated_at, deleted_at
				 FROM domains
				 WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"workspace_id": *workspaceID, "after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
				        dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error,
				        created_at, updated_at, deleted_at
				 FROM domains
				 WHERE workspace_id = @workspace_id AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"workspace_id": *workspaceID, "limit": fetchLimit},
			)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("listing domains by workspace: %w", err)
	}

	domains, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Domain])
	if err != nil {
		return nil, fmt.Errorf("collecting domains: %w", err)
	}

	result := &port.PageResult[domain.Domain]{
		Items:   domains,
		HasMore: len(domains) > limit,
	}
	if result.HasMore {
		result.Items = domains[:limit]
		result.NextCursor = EncodeCursor(domains[limit-1].ID)
	}

	return result, nil
}

func (r *DomainRepo) GetPendingVerifications(ctx context.Context, limit int) ([]*domain.Domain, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, domain_name, status, dkim_selector, dkim_private_key_encrypted,
		        dkim_public_key, dns_records, verified_at, last_check_at, next_check_at, last_error,
		        created_at, updated_at, deleted_at
		 FROM domains
		 WHERE status != 'verified' AND deleted_at IS NULL
		   AND (next_check_at IS NULL OR next_check_at <= now())
		 ORDER BY next_check_at NULLS FIRST
		 LIMIT @limit`,
		pgx.NamedArgs{"limit": limit},
	)
	if err != nil {
		return nil, fmt.Errorf("querying pending verifications: %w", err)
	}

	domains, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Domain])
	if err != nil {
		return nil, fmt.Errorf("collecting pending domains: %w", err)
	}

	return domains, nil
}

func scanDomain(row pgx.Row) (*domain.Domain, error) {
	var d domain.Domain
	err := row.Scan(
		&d.ID, &d.WorkspaceID, &d.DomainName, &d.Status,
		&d.DKIMSelector, &d.DKIMPrivateKeyEncrypted, &d.DKIMPublicKey,
		&d.DNSRecords, &d.VerifiedAt, &d.LastCheckAt, &d.NextCheckAt, &d.LastError,
		&d.CreatedAt, &d.UpdatedAt, &d.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("domain not found")
		}
		return nil, fmt.Errorf("scanning domain: %w", err)
	}
	return &d, nil
}

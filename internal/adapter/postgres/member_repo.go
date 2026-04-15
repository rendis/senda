package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
)

// MemberRepo implements port.MemberStore using PostgreSQL.
type MemberRepo struct {
	pool *pgxpool.Pool
}

// NewMemberRepo creates a new MemberRepo.
func NewMemberRepo(pool *pgxpool.Pool) *MemberRepo {
	return &MemberRepo{pool: pool}
}

func (r *MemberRepo) Create(ctx context.Context, member *domain.Member) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO members (id, email, display_name, oidc_subject, oidc_issuer)
		 VALUES (@id, @email, @display_name, @oidc_subject, @oidc_issuer)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":           member.ID,
			"email":        member.Email,
			"display_name": member.DisplayName,
			"oidc_subject": member.OIDCSubject,
			"oidc_issuer":  member.OIDCIssuer,
		},
	)

	if err := row.Scan(&member.CreatedAt, &member.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting member: %w", err)
	}

	return nil
}

func (r *MemberRepo) GetByEmail(ctx context.Context, email string) (*domain.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email, display_name, oidc_subject, oidc_issuer, created_at, updated_at
		 FROM members
		 WHERE email = @email`,
		pgx.NamedArgs{"email": email},
	)
	if err != nil {
		return nil, fmt.Errorf("querying member by email: %w", err)
	}

	member, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[domain.Member])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("member with email %q not found", email)
		}
		return nil, fmt.Errorf("scanning member: %w", err)
	}

	return member, nil
}

func (r *MemberRepo) GetByOIDCIdentity(ctx context.Context, issuer, subject string) (*domain.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email, display_name, oidc_subject, oidc_issuer, created_at, updated_at
		 FROM members
		 WHERE oidc_issuer = @issuer
		   AND oidc_subject = @subject`,
		pgx.NamedArgs{
			"issuer":  issuer,
			"subject": subject,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("querying member by oidc identity: %w", err)
	}

	member, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[domain.Member])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("member with oidc issuer %q and subject %q not found", issuer, subject)
		}
		return nil, fmt.Errorf("scanning member: %w", err)
	}

	return member, nil
}

func (r *MemberRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email, display_name, oidc_subject, oidc_issuer, created_at, updated_at
		 FROM members
		 WHERE id = @id`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return nil, fmt.Errorf("querying member by id: %w", err)
	}

	member, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[domain.Member])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("member %s not found", id)
		}
		return nil, fmt.Errorf("scanning member: %w", err)
	}

	return member, nil
}

func (r *MemberRepo) ListAll(ctx context.Context, opts port.ListOptions) ([]*domain.Member, string, error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, "", err
	}

	fetchLimit := limit + 1

	var rows pgx.Rows
	if afterID != nil {
		rows, err = r.pool.Query(ctx,
			`SELECT id, email, display_name, oidc_subject, oidc_issuer, created_at, updated_at
			 FROM members
			 WHERE id < @after_id
			 ORDER BY id DESC
			 LIMIT @limit`,
			pgx.NamedArgs{"after_id": *afterID, "limit": fetchLimit},
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, email, display_name, oidc_subject, oidc_issuer, created_at, updated_at
			 FROM members
			 ORDER BY id DESC
			 LIMIT @limit`,
			pgx.NamedArgs{"limit": fetchLimit},
		)
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing members: %w", err)
	}

	members, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Member])
	if err != nil {
		return nil, "", fmt.Errorf("collecting members: %w", err)
	}

	var nextCursor string
	if len(members) > limit {
		members = members[:limit]
		nextCursor = EncodeCursor(members[limit-1].ID)
	}

	return members, nextCursor, nil
}

func (r *MemberRepo) ListInScope(ctx context.Context, scopeType domain.ScopeType, scopeID *uuid.UUID, opts port.ListOptions) ([]*domain.Member, string, error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, "", err
	}

	if scopeType != domain.ScopeTenant && scopeType != domain.ScopeWorkspace {
		return nil, "", fmt.Errorf("unsupported member scope %q", scopeType)
	}
	if scopeID == nil {
		return nil, "", fmt.Errorf("scope id is required for scoped member listing")
	}

	fetchLimit := limit + 1
	args := pgx.NamedArgs{
		"scope_type": scopeType,
		"scope_id":   *scopeID,
		"limit":      fetchLimit,
	}

	query := `
		SELECT id, email, display_name, oidc_subject, oidc_issuer, created_at, updated_at
		FROM members m
		WHERE EXISTS (
			SELECT 1
			FROM member_roles mr
			WHERE mr.member_id = m.id
			  AND mr.scope_type = @scope_type`

	switch scopeType {
	case domain.ScopeTenant:
		query += " AND mr.tenant_id = @scope_id"
	case domain.ScopeWorkspace:
		query += " AND mr.workspace_id = @scope_id"
	}

	query += `
		)
	`
	if afterID != nil {
		query += " AND id < @after_id"
		args["after_id"] = *afterID
	}
	query += `
		ORDER BY id DESC
		LIMIT @limit`

	rows, err := r.pool.Query(ctx, query, args)
	if err != nil {
		return nil, "", fmt.Errorf("listing members by scope: %w", err)
	}

	members, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.Member])
	if err != nil {
		return nil, "", fmt.Errorf("collecting scoped members: %w", err)
	}

	var nextCursor string
	if len(members) > limit {
		members = members[:limit]
		nextCursor = EncodeCursor(members[limit-1].ID)
	}

	return members, nextCursor, nil
}

func (r *MemberRepo) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM members`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting members: %w", err)
	}
	return count, nil
}

func (r *MemberRepo) AddRole(ctx context.Context, role *domain.MemberRole) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO member_roles (id, member_id, role, scope_type, tenant_id, workspace_id, created_by)
		 VALUES (@id, @member_id, @role, @scope_type, @tenant_id, @workspace_id, @created_by)
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":           role.ID,
			"member_id":    role.MemberID,
			"role":         role.Role,
			"scope_type":   role.ScopeType,
			"tenant_id":    role.TenantID,
			"workspace_id": role.WorkspaceID,
			"created_by":   role.CreatedBy,
		},
	)

	if err := row.Scan(&role.CreatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return apperr.Validation("invalid role/scope combination")
		}
		return fmt.Errorf("inserting member role: %w", err)
	}

	return nil
}

func (r *MemberRepo) ReplaceRoleInScope(ctx context.Context, role *domain.MemberRole) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin replace member role tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := getRolesInScopeQuerier(ctx, tx, role.MemberID, role.ScopeType, scopeIDForRole(role))
	if err != nil {
		return err
	}

	if len(existing) > 1 {
		slices.SortFunc(existing, compareMemberRolesForScope)
	}

	if len(existing) == 1 && sameLocalScopeRole(existing[0], role) {
		role.ID = existing[0].ID
		role.CreatedAt = existing[0].CreatedAt
		role.CreatedBy = existing[0].CreatedBy
		role.TenantID = existing[0].TenantID
		role.WorkspaceID = existing[0].WorkspaceID
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit replace member role tx: %w", err)
		}
		return nil
	}

	if len(existing) == 0 {
		if err := insertMemberRoleQuerier(ctx, tx, role); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit replace member role tx: %w", err)
		}
		return nil
	}

	kept := existing[0]
	if _, err := tx.Exec(ctx,
		`UPDATE member_roles
		 SET role = @role
		 WHERE id = @id`,
		pgx.NamedArgs{
			"id":   kept.ID,
			"role": role.Role,
		},
	); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return apperr.Validation("invalid role/scope combination")
		}
		return fmt.Errorf("updating member role in scope: %w", err)
	}

	if len(existing) > 1 {
		extraIDs := make([]uuid.UUID, 0, len(existing)-1)
		for _, existingRole := range existing[1:] {
			extraIDs = append(extraIDs, existingRole.ID)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM member_roles WHERE id = ANY(@ids)`,
			pgx.NamedArgs{"ids": extraIDs},
		); err != nil {
			return fmt.Errorf("deleting superseded member roles: %w", err)
		}
	}

	role.ID = kept.ID
	role.CreatedAt = kept.CreatedAt
	role.CreatedBy = kept.CreatedBy
	role.TenantID = kept.TenantID
	role.WorkspaceID = kept.WorkspaceID

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit replace member role tx: %w", err)
	}

	return nil
}

func (r *MemberRepo) RemoveRole(ctx context.Context, roleID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM member_roles WHERE id = @id`,
		pgx.NamedArgs{"id": roleID},
	)
	if err != nil {
		return fmt.Errorf("deleting member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("member role %s not found", roleID)
	}
	return nil
}

func (r *MemberRepo) RevokeAccessInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error) {
	var (
		count int64
		query string
		args  pgx.NamedArgs
	)

	switch scopeType {
	case domain.ScopeGlobal:
		query = `
			WITH revoked AS (
				DELETE FROM member_roles
				WHERE member_id = @member_id
				  AND scope_type = 'global'
				RETURNING 1
			)
			SELECT COUNT(*) FROM revoked`
		args = pgx.NamedArgs{"member_id": memberID}
	case domain.ScopeTenant:
		if scopeID == nil {
			return 0, fmt.Errorf("scope id is required for tenant access revocation")
		}
		query = `
			WITH revoked AS (
				DELETE FROM member_roles
				WHERE member_id = @member_id
				  AND scope_type = 'tenant'
				  AND tenant_id = @scope_id
				RETURNING 1
			)
			SELECT COUNT(*) FROM revoked`
		args = pgx.NamedArgs{"member_id": memberID, "scope_id": *scopeID}
	case domain.ScopeWorkspace:
		if scopeID == nil {
			return 0, fmt.Errorf("scope id is required for workspace access revocation")
		}
		query = `
			WITH revoked AS (
				DELETE FROM member_roles
				WHERE member_id = @member_id
				  AND scope_type = 'workspace'
				  AND workspace_id = @scope_id
				RETURNING 1
			)
			SELECT COUNT(*) FROM revoked`
		args = pgx.NamedArgs{"member_id": memberID, "scope_id": *scopeID}
	default:
		return 0, fmt.Errorf("unknown scope type: %s", scopeType)
	}

	if err := r.pool.QueryRow(ctx, query, args).Scan(&count); err != nil {
		return 0, fmt.Errorf("revoking member access: %w", err)
	}

	return count, nil
}

func (r *MemberRepo) GetRoles(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
		 FROM member_roles
		 WHERE member_id = @member_id
		 ORDER BY created_at`,
		pgx.NamedArgs{"member_id": memberID},
	)
	if err != nil {
		return nil, fmt.Errorf("querying member roles: %w", err)
	}

	roles, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.MemberRole])
	if err != nil {
		return nil, fmt.Errorf("collecting member roles: %w", err)
	}

	return roles, nil
}

func (r *MemberRepo) GetRolesByMembers(ctx context.Context, memberIDs []uuid.UUID) (map[uuid.UUID][]*domain.MemberRole, error) { //nolint:dupl // structurally similar to InjectorRepo.GetAllFieldsByDefinitions
	if len(memberIDs) == 0 {
		return make(map[uuid.UUID][]*domain.MemberRole), nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
		 FROM member_roles
		 WHERE member_id = ANY(@member_ids)
		 ORDER BY member_id, created_at`,
		pgx.NamedArgs{"member_ids": memberIDs},
	)
	if err != nil {
		return nil, fmt.Errorf("batch querying member roles: %w", err)
	}

	roles, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.MemberRole])
	if err != nil {
		return nil, fmt.Errorf("collecting batch member roles: %w", err)
	}

	result := make(map[uuid.UUID][]*domain.MemberRole, len(memberIDs))
	for _, r := range roles {
		result[r.MemberID] = append(result[r.MemberID], r)
	}
	return result, nil
}

func (r *MemberRepo) GetRolesInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
	return getRolesInScopeQuerier(ctx, r.pool, memberID, scopeType, scopeID)
}

func getRolesInScopeQuerier(ctx context.Context, querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
	var rows pgx.Rows
	var err error

	switch scopeType {
	case domain.ScopeGlobal:
		rows, err = querier.Query(ctx,
			`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
			 FROM member_roles
			 WHERE member_id = @member_id AND scope_type = 'global'`,
			pgx.NamedArgs{"member_id": memberID},
		)
	case domain.ScopeTenant:
		if scopeID == nil {
			return nil, fmt.Errorf("scope id is required for tenant member role query")
		}
		rows, err = querier.Query(ctx,
			`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
			 FROM member_roles
			 WHERE member_id = @member_id AND scope_type = 'tenant' AND tenant_id = @scope_id`,
			pgx.NamedArgs{"member_id": memberID, "scope_id": *scopeID},
		)
	case domain.ScopeWorkspace:
		if scopeID == nil {
			return nil, fmt.Errorf("scope id is required for workspace member role query")
		}
		rows, err = querier.Query(ctx,
			`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
			 FROM member_roles
			 WHERE member_id = @member_id AND scope_type = 'workspace' AND workspace_id = @scope_id`,
			pgx.NamedArgs{"member_id": memberID, "scope_id": *scopeID},
		)
	default:
		return nil, fmt.Errorf("unknown scope type: %s", scopeType)
	}

	if err != nil {
		return nil, fmt.Errorf("querying member roles in scope: %w", err)
	}

	roles, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.MemberRole])
	if err != nil {
		return nil, fmt.Errorf("collecting member roles: %w", err)
	}

	return roles, nil
}

func insertMemberRoleQuerier(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, role *domain.MemberRole) error {
	row := querier.QueryRow(ctx,
		`INSERT INTO member_roles (id, member_id, role, scope_type, tenant_id, workspace_id, created_by)
		 VALUES (@id, @member_id, @role, @scope_type, @tenant_id, @workspace_id, @created_by)
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":           role.ID,
			"member_id":    role.MemberID,
			"role":         role.Role,
			"scope_type":   role.ScopeType,
			"tenant_id":    role.TenantID,
			"workspace_id": role.WorkspaceID,
			"created_by":   role.CreatedBy,
		},
	)

	if err := row.Scan(&role.CreatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return apperr.Validation("invalid role/scope combination")
		}
		return fmt.Errorf("inserting member role: %w", err)
	}

	return nil
}

func scopeIDForRole(role *domain.MemberRole) *uuid.UUID {
	switch role.ScopeType {
	case domain.ScopeTenant:
		return role.TenantID
	case domain.ScopeWorkspace:
		return role.WorkspaceID
	default:
		return nil
	}
}

func compareMemberRolesForScope(a, b *domain.MemberRole) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	if cmp := b.Role.Level() - a.Role.Level(); cmp != 0 {
		return cmp
	}
	if a.CreatedAt.Equal(b.CreatedAt) {
		switch {
		case a.ID == b.ID:
			return 0
		case a.ID.String() < b.ID.String():
			return -1
		default:
			return 1
		}
	}
	if a.CreatedAt.After(b.CreatedAt) {
		return -1
	}
	return 1
}

func sameLocalScopeRole(existing, desired *domain.MemberRole) bool {
	if existing == nil || desired == nil {
		return false
	}
	if existing.Role != desired.Role || existing.ScopeType != desired.ScopeType {
		return false
	}
	switch desired.ScopeType {
	case domain.ScopeGlobal:
		return true
	case domain.ScopeTenant:
		return existing.TenantID != nil && desired.TenantID != nil && *existing.TenantID == *desired.TenantID
	case domain.ScopeWorkspace:
		return existing.WorkspaceID != nil && desired.WorkspaceID != nil && *existing.WorkspaceID == *desired.WorkspaceID
	default:
		return false
	}
}

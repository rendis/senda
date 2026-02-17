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
	"github.com/senda-app/senda/pkg/apperr"
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperr.Conflict("member with email %q already exists", member.Email)
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return apperr.Conflict("duplicate role assignment")
			case "23514":
				return apperr.Validation("invalid role/scope combination")
			}
		}
		return fmt.Errorf("inserting member role: %w", err)
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

func (r *MemberRepo) GetRolesInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error) {
	var rows pgx.Rows
	var err error

	switch scopeType {
	case domain.ScopeGlobal:
		rows, err = r.pool.Query(ctx,
			`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
			 FROM member_roles
			 WHERE member_id = @member_id AND scope_type = 'global'`,
			pgx.NamedArgs{"member_id": memberID},
		)
	case domain.ScopeTenant:
		rows, err = r.pool.Query(ctx,
			`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
			 FROM member_roles
			 WHERE member_id = @member_id AND scope_type = 'tenant' AND tenant_id = @scope_id`,
			pgx.NamedArgs{"member_id": memberID, "scope_id": scopeID},
		)
	case domain.ScopeWorkspace:
		rows, err = r.pool.Query(ctx,
			`SELECT id, member_id, role, scope_type, tenant_id, workspace_id, created_at, created_by
			 FROM member_roles
			 WHERE member_id = @member_id AND scope_type = 'workspace' AND workspace_id = @scope_id`,
			pgx.NamedArgs{"member_id": memberID, "scope_id": scopeID},
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

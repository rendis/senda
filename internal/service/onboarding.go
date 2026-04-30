package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// TxBeginner abstracts transaction creation so the service can be tested
// without a real pgxpool.Pool.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// OnboardingRequest holds the data needed to set up the first tenant.
type OnboardingRequest struct {
	TenantCode string
	TenantName string
}

// OnboardingResult holds the entities created during onboarding.
type OnboardingResult struct {
	Member    *domain.Member
	Tenant    *domain.Tenant
	Workspace *domain.Workspace
}

// OnboardingService handles the first-use onboarding flow.
type OnboardingService struct {
	pool        TxBeginner
	memberStore port.MemberStore
	tenantStore port.TenantStore
	wsStore     port.WorkspaceStore
	auditStore  port.AuditLogStore
}

// NewOnboardingService creates a new OnboardingService.
// pool can be *pgxpool.Pool or any TxBeginner implementation.
func NewOnboardingService(
	pool TxBeginner,
	ms port.MemberStore,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	as port.AuditLogStore,
) *OnboardingService {
	return &OnboardingService{
		pool:        pool,
		memberStore: ms,
		tenantStore: ts,
		wsStore:     ws,
		auditStore:  as,
	}
}

// Status checks if onboarding is needed (no members exist).
func (s *OnboardingService) Status(ctx context.Context) (bool, error) {
	count, err := s.memberStore.CountAll(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// Setup creates the first member + superadmin role + tenant_admin role + tenant + _system workspace.
// Guard: only works when CountAll() == 0, else returns domain.ErrConflict.
//
// Concurrency safety: acquires a PostgreSQL advisory lock (pg_advisory_xact_lock)
// within a transaction to serialize all concurrent Setup calls. This eliminates
// the TOCTOU race between CountAll and Create. The lock is automatically released
// when the transaction commits or rolls back.
func (s *OnboardingService) Setup(ctx context.Context, claims *port.OIDCClaims, req *OnboardingRequest) (*OnboardingResult, error) { //nolint:funlen // transactional onboarding with multiple entity creation
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Advisory lock — serializes all concurrent setup calls.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('senda_onboarding'))"); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}

	// All subsequent operations run on tx (not the pool) for full atomicity.
	// If any step fails, the deferred tx.Rollback undoes everything.

	var count int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM members`).Scan(&count); err != nil {
		return nil, fmt.Errorf("count members: %w", err)
	}
	if count != 0 {
		return nil, domain.ErrConflict
	}

	now := time.Now().UTC()

	// 1. Create member from OIDC claims.
	member := &domain.Member{
		ID:          uuid.Must(uuid.NewV7()),
		Email:       claims.Email,
		OIDCSubject: &claims.Subject,
		OIDCIssuer:  &claims.Issuer,
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO members (id, email, oidc_subject, oidc_issuer)
		 VALUES (@id, @email, @oidc_subject, @oidc_issuer)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":           member.ID,
			"email":        member.Email,
			"oidc_subject": member.OIDCSubject,
			"oidc_issuer":  member.OIDCIssuer,
		},
	).Scan(&member.CreatedAt, &member.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create member: %w", err)
	}

	// 2. Add superadmin role (global scope).
	superadminRole := &domain.MemberRole{
		ID:        uuid.Must(uuid.NewV7()),
		MemberID:  member.ID,
		Role:      domain.RoleSuperadmin,
		ScopeType: domain.ScopeGlobal,
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO member_roles (id, member_id, role, scope_type)
		 VALUES (@id, @member_id, @role, @scope_type)
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":         superadminRole.ID,
			"member_id":  superadminRole.MemberID,
			"role":       superadminRole.Role,
			"scope_type": superadminRole.ScopeType,
		},
	).Scan(&superadminRole.CreatedAt); err != nil {
		return nil, fmt.Errorf("add superadmin role: %w", err)
	}

	// 3. Create tenant.
	tenant := &domain.Tenant{
		ID:       uuid.Must(uuid.NewV7()),
		Code:     req.TenantCode,
		Name:     req.TenantName,
		IsActive: true,
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO tenants (id, code, name, is_active)
		 VALUES (@id, @code, @name, @is_active)
		 RETURNING is_active, created_at, updated_at`,
		pgx.NamedArgs{
			"id":        tenant.ID,
			"code":      tenant.Code,
			"name":      tenant.Name,
			"is_active": tenant.IsActive,
		},
	).Scan(&tenant.IsActive, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	// 4. Add tenant_admin role (tenant scope).
	tenantAdminRole := &domain.MemberRole{
		ID:        uuid.Must(uuid.NewV7()),
		MemberID:  member.ID,
		Role:      domain.RoleTenantAdmin,
		ScopeType: domain.ScopeTenant,
		TenantID:  &tenant.ID,
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO member_roles (id, member_id, role, scope_type, tenant_id)
		 VALUES (@id, @member_id, @role, @scope_type, @tenant_id)
		 RETURNING created_at`,
		pgx.NamedArgs{
			"id":         tenantAdminRole.ID,
			"member_id":  tenantAdminRole.MemberID,
			"role":       tenantAdminRole.Role,
			"scope_type": tenantAdminRole.ScopeType,
			"tenant_id":  tenantAdminRole.TenantID,
		},
	).Scan(&tenantAdminRole.CreatedAt); err != nil {
		return nil, fmt.Errorf("add tenant admin role: %w", err)
	}

	// 5. Create _system logical workspace pair.
	logicalWorkspaceID := uuid.Must(uuid.NewV7())
	ws := &domain.Workspace{
		ID:                                   uuid.Must(uuid.NewV7()),
		LogicalWorkspaceID:                   logicalWorkspaceID,
		TenantID:                             tenant.ID,
		Code:                                 "_system",
		Name:                                 "System",
		Environment:                          domain.EnvironmentProd,
		IsSystem:                             true,
		IsActive:                             true,
		AllowWorkspaceLocalTemplates:         true,
		AllowWorkspaceInheritedTemplateForks: true,
		AllowWorkspaceLocalInjectors:         true,
		WorkspacePoliciesInitialized:         true,
	}
	testWS := &domain.Workspace{
		ID:                                   uuid.Must(uuid.NewV7()),
		LogicalWorkspaceID:                   logicalWorkspaceID,
		TenantID:                             tenant.ID,
		Code:                                 "_system",
		Name:                                 "System",
		Environment:                          domain.EnvironmentTest,
		IsSystem:                             true,
		IsActive:                             true,
		AllowWorkspaceLocalTemplates:         true,
		AllowWorkspaceInheritedTemplateForks: true,
		AllowWorkspaceLocalInjectors:         true,
		WorkspacePoliciesInitialized:         true,
	}
	for _, currentWorkspace := range []*domain.Workspace{ws, testWS} {
		wsSigningKey := make([]byte, 32)
		if _, err := rand.Read(wsSigningKey); err != nil {
			return nil, fmt.Errorf("generate signing key for system workspace (%s): %w", currentWorkspace.Environment, err)
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO workspaces (
			    id, logical_workspace_id, tenant_id, code, name, environment, is_system, is_active, open_tracking_enabled, default_locale,
			    allow_workspace_local_templates, allow_workspace_inherited_template_forks, allow_workspace_local_injectors,
			    unsubscribe_signing_key
			)
			 VALUES (
			    @id, @logical_workspace_id, @tenant_id, @code, @name, @environment, @is_system, @is_active, @open_tracking_enabled, @default_locale,
			    @allow_workspace_local_templates, @allow_workspace_inherited_template_forks, @allow_workspace_local_injectors,
			    @unsubscribe_signing_key
			)
			 RETURNING is_active, created_at, updated_at`,
			pgx.NamedArgs{
				"id":                              currentWorkspace.ID,
				"logical_workspace_id":            currentWorkspace.LogicalWorkspaceID,
				"tenant_id":                       currentWorkspace.TenantID,
				"code":                            currentWorkspace.Code,
				"name":                            currentWorkspace.Name,
				"environment":                     currentWorkspace.Environment,
				"is_system":                       currentWorkspace.IsSystem,
				"is_active":                       currentWorkspace.IsActive,
				"open_tracking_enabled":           false,
				"default_locale":                  currentWorkspace.DefaultLocale,
				"allow_workspace_local_templates": currentWorkspace.AllowWorkspaceLocalTemplates,
				"allow_workspace_inherited_template_forks": currentWorkspace.AllowWorkspaceInheritedTemplateForks,
				"allow_workspace_local_injectors":          currentWorkspace.AllowWorkspaceLocalInjectors,
				"unsubscribe_signing_key":                  wsSigningKey,
			},
		).Scan(&currentWorkspace.IsActive, &currentWorkspace.CreatedAt, &currentWorkspace.UpdatedAt); err != nil {
			return nil, fmt.Errorf("create system workspace (%s): %w", currentWorkspace.Environment, err)
		}
	}

	// 6. Audit log (best-effort with savepoint — a failed INSERT must not
	//    abort the outer TX or the subsequent COMMIT will rollback everything).
	if _, spErr := tx.Exec(ctx, "SAVEPOINT audit_sp"); spErr == nil {
		changesJSON, _ := json.Marshal(map[string]any{"tenant_code": tenant.Code, "tenant_name": tenant.Name, "member_email": member.Email})
		_, auditErr := tx.Exec(ctx,
			`INSERT INTO audit_logs (id, member_id, member_email, action, resource_type, resource_id, tenant_id, scope_type, changes, created_at)
			 VALUES (@id, @member_id, @member_email, @action, @resource_type, @resource_id, @tenant_id, @scope_type, @changes::jsonb, @created_at)`,
			pgx.NamedArgs{
				"id":            uuid.Must(uuid.NewV7()),
				"member_id":     member.ID,
				"member_email":  member.Email,
				"action":        domain.AuditCreate,
				"resource_type": "onboarding",
				"resource_id":   tenant.ID,
				"tenant_id":     &tenant.ID,
				"scope_type":    domain.ScopeGlobal,
				"changes":       string(changesJSON),
				"created_at":    now,
			},
		)
		if auditErr != nil {
			_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT audit_sp")
		} else {
			_, _ = tx.Exec(ctx, "RELEASE SAVEPOINT audit_sp")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &OnboardingResult{
		Member:    member,
		Tenant:    tenant,
		Workspace: ws,
	}, nil
}

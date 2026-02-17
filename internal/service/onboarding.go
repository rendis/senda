package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

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
	memberStore port.MemberStore
	tenantStore port.TenantStore
	wsStore     port.WorkspaceStore
	auditStore  port.AuditLogStore
}

// NewOnboardingService creates a new OnboardingService.
func NewOnboardingService(
	ms port.MemberStore,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	as port.AuditLogStore,
) *OnboardingService {
	return &OnboardingService{
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
// TOCTOU mitigation: after creating the member, re-check the count. If another
// concurrent Setup call raced and also created a member, the count will be > 1
// and we return ErrConflict. The DB's unique constraints on tenant code and
// member email provide an additional safety net.
func (s *OnboardingService) Setup(ctx context.Context, claims *port.OIDCClaims, req *OnboardingRequest) (*OnboardingResult, error) {
	count, err := s.memberStore.CountAll(ctx)
	if err != nil {
		return nil, err
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
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.memberStore.Create(ctx, member); err != nil {
		// If member creation fails due to unique constraint (concurrent setup),
		// treat as conflict.
		return nil, err
	}

	// Re-check count after member creation to detect concurrent setup race.
	// If another goroutine also passed the initial check and created a member,
	// count will now be > 1.
	postCount, err := s.memberStore.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	if postCount > 1 {
		return nil, domain.ErrConflict
	}

	// 2. Add superadmin role (global scope).
	superadminRole := &domain.MemberRole{
		ID:        uuid.Must(uuid.NewV7()),
		MemberID:  member.ID,
		Role:      domain.RoleSuperadmin,
		ScopeType: domain.ScopeGlobal,
		CreatedAt: now,
	}
	if err := s.memberStore.AddRole(ctx, superadminRole); err != nil {
		return nil, err
	}

	// 3. Create tenant.
	tenant := &domain.Tenant{
		ID:        uuid.Must(uuid.NewV7()),
		Code:      req.TenantCode,
		Name:      req.TenantName,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.tenantStore.Create(ctx, tenant); err != nil {
		return nil, err
	}

	// 4. Add tenant_admin role (tenant scope).
	tenantAdminRole := &domain.MemberRole{
		ID:        uuid.Must(uuid.NewV7()),
		MemberID:  member.ID,
		Role:      domain.RoleTenantAdmin,
		ScopeType: domain.ScopeTenant,
		TenantID:  &tenant.ID,
		CreatedAt: now,
	}
	if err := s.memberStore.AddRole(ctx, tenantAdminRole); err != nil {
		return nil, err
	}

	// 5. Create _system workspace.
	ws := &domain.Workspace{
		ID:        uuid.Must(uuid.NewV7()),
		TenantID:  tenant.ID,
		Code:      "_system",
		Name:      "System",
		IsSystem:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.wsStore.Create(ctx, ws); err != nil {
		return nil, err
	}

	// 6. Audit log.
	auditEntry := &domain.AuditLog{
		ID:         uuid.Must(uuid.NewV7()),
		ActorID:    member.ID,
		ActorEmail: member.Email,
		Action:     domain.AuditCreate,
		EntityType: "onboarding",
		EntityID:   tenant.ID,
		TenantID:   &tenant.ID,
		ScopeType:  domain.ScopeGlobal,
		Changes: map[string]any{
			"tenant_code":  tenant.Code,
			"tenant_name":  tenant.Name,
			"member_email": member.Email,
		},
		CreatedAt: now,
	}
	// Audit log failures should not block onboarding.
	_ = s.auditStore.Append(ctx, auditEntry)

	return &OnboardingResult{
		Member:    member,
		Tenant:    tenant,
		Workspace: ws,
	}, nil
}

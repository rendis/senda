package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// AdapterAccess describes how a workspace can see an adapter.
type AdapterAccess struct {
	Adapter    *domain.Adapter
	Shared     bool
	Editable   bool
	OwnerScope *domain.Workspace
}

// WorkspaceAccessGrant is the workspace-facing representation of a grant toggle.
type WorkspaceAccessGrant struct {
	Workspace *domain.Workspace
	Granted   bool
}

// AdapterAccessService centralizes adapter/identity sharing rules.
type AdapterAccessService struct {
	adapterStore       port.AdapterStore
	identityStore      port.AdapterIdentityStore
	workspaceStore     port.WorkspaceStore
	adapterGrants      port.AdapterGrantStore
	identityGrants     port.AdapterIdentityGrantStore
	templateTypeUsages port.TemplateTypeUsageStore
}

// NewAdapterAccessService creates a new AdapterAccessService.
func NewAdapterAccessService(
	adapterStore port.AdapterStore,
	identityStore port.AdapterIdentityStore,
	workspaceStore port.WorkspaceStore,
	adapterGrants port.AdapterGrantStore,
	identityGrants port.AdapterIdentityGrantStore,
	templateTypeUsages port.TemplateTypeUsageStore,
) *AdapterAccessService {
	return &AdapterAccessService{
		adapterStore:       adapterStore,
		identityStore:      identityStore,
		workspaceStore:     workspaceStore,
		adapterGrants:      adapterGrants,
		identityGrants:     identityGrants,
		templateTypeUsages: templateTypeUsages,
	}
}

// ListAdaptersForWorkspace returns adapters owned by the workspace plus shared system adapters visible to it.
func (s *AdapterAccessService) ListAdaptersForWorkspace(ctx context.Context, workspace *domain.Workspace, opts port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	if workspace == nil {
		return s.adapterStore.ListByWorkspace(ctx, nil, opts)
	}
	if workspace.IsSystem || s.adapterGrants == nil {
		return s.adapterStore.ListByWorkspace(ctx, &workspace.ID, opts)
	}
	return s.adapterGrants.ListVisibleAdaptersForWorkspace(ctx, workspace.ID, opts)
}

// GetAdapterAccess resolves whether a workspace can see/use an adapter.
func (s *AdapterAccessService) GetAdapterAccess(ctx context.Context, workspace *domain.Workspace, adapterID uuid.UUID) (*AdapterAccess, error) {
	adapter, err := s.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}

	if workspace == nil {
		if adapter.WorkspaceID == nil {
			return &AdapterAccess{Adapter: adapter, Editable: true}, nil
		}
		return nil, domain.ErrAdapterAccessDenied
	}

	if adapter.WorkspaceID != nil && *adapter.WorkspaceID == workspace.ID {
		return &AdapterAccess{Adapter: adapter, Editable: true}, nil
	}

	if adapter.WorkspaceID == nil {
		return &AdapterAccess{Adapter: adapter, Editable: false}, nil
	}

	owner, err := s.workspaceStore.GetByID(ctx, *adapter.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if owner.TenantID != workspace.TenantID || !owner.IsSystem {
		return nil, domain.ErrAdapterAccessDenied
	}

	switch {
	case adapter.AdapterType == domain.AdapterTypeGmail:
		if s.adapterGrants == nil {
			return nil, domain.ErrAdapterAccessDenied
		}
		granted, err := s.adapterGrants.HasAdapterWorkspaceGrant(ctx, adapter.ID, workspace.ID)
		if err != nil {
			return nil, err
		}
		if !granted {
			return nil, domain.ErrAdapterAccessDenied
		}
	case usesIdentityGrants(adapter.AdapterType):
		if s.identityGrants == nil {
			return nil, domain.ErrAdapterAccessDenied
		}
		identities, err := s.identityGrants.ListGrantedIdentitiesForWorkspace(ctx, adapter.ID, workspace.ID)
		if err != nil {
			return nil, err
		}
		if len(identities) == 0 {
			return nil, domain.ErrAdapterAccessDenied
		}
	default:
		return nil, domain.ErrAdapterAccessDenied
	}

	return &AdapterAccess{Adapter: adapter, Shared: true, Editable: false, OwnerScope: owner}, nil
}

// ListIdentitiesForWorkspace returns all identities for owned/system scope and only granted emails for identity-scoped adapters.
func (s *AdapterAccessService) ListIdentitiesForWorkspace(ctx context.Context, workspace *domain.Workspace, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	if workspace == nil || workspace.IsSystem {
		return s.identityStore.ListByAdapter(ctx, adapterID)
	}

	access, err := s.GetAdapterAccess(ctx, workspace, adapterID)
	if err != nil {
		return nil, err
	}
	if access.Editable || access.Adapter.AdapterType == domain.AdapterTypeGmail {
		return s.identityStore.ListByAdapter(ctx, adapterID)
	}
	return s.identityGrants.ListGrantedIdentitiesForWorkspace(ctx, adapterID, workspace.ID)
}

// ValidateTemplateTypeSelection validates adapter/sender selection for a workspace-owned template type.
func (s *AdapterAccessService) ValidateTemplateTypeSelection(ctx context.Context, workspace *domain.Workspace, adapterID, senderIdentityID *uuid.UUID) error {
	if adapterID == nil || workspace == nil || workspace.IsSystem {
		return nil
	}

	adapter, err := s.adapterStore.GetByID(ctx, *adapterID)
	if err != nil {
		return err
	}
	if adapter.WorkspaceID == nil || (adapter.WorkspaceID != nil && *adapter.WorkspaceID == workspace.ID) {
		return s.validateSenderIdentity(ctx, adapter, senderIdentityID, nil)
	}

	owner, err := s.workspaceStore.GetByID(ctx, *adapter.WorkspaceID)
	if err != nil {
		return err
	}
	if owner.TenantID != workspace.TenantID || !owner.IsSystem {
		return domain.ErrAdapterAccessDenied
	}

	switch {
	case adapter.AdapterType == domain.AdapterTypeGmail:
		granted, err := s.adapterGrants.HasAdapterWorkspaceGrant(ctx, adapter.ID, workspace.ID)
		if err != nil {
			return err
		}
		if !granted {
			return domain.ErrAdapterAccessDenied
		}
		return s.validateSenderIdentity(ctx, adapter, senderIdentityID, nil)
	case usesIdentityGrants(adapter.AdapterType):
		if senderIdentityID == nil {
			return domain.ErrSenderIdentityRequired
		}
		return s.validateSenderIdentity(ctx, adapter, senderIdentityID, func(identity *domain.AdapterIdentity) error {
			granted, err := s.identityGrants.HasIdentityWorkspaceGrant(ctx, identity.ID, workspace.ID)
			if err != nil {
				return err
			}
			if !granted {
				return domain.ErrSenderIdentityAccessDenied
			}
			return nil
		})
	default:
		return domain.ErrAdapterAccessDenied
	}
}

// ReplaceAdapterWorkspaceAccess replaces the workspace grant set for a shared Gmail adapter.
func (s *AdapterAccessService) ReplaceAdapterWorkspaceAccess(ctx context.Context, systemWorkspace *domain.Workspace, adapterID uuid.UUID, workspaceIDs []uuid.UUID) error {
	if systemWorkspace == nil || !systemWorkspace.IsSystem {
		return fmt.Errorf("%w: adapter grants can only be managed from _system", domain.ErrForbidden)
	}
	adapter, err := s.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return err
	}
	if adapter.WorkspaceID == nil || *adapter.WorkspaceID != systemWorkspace.ID || adapter.AdapterType != domain.AdapterTypeGmail {
		return fmt.Errorf("%w: only system-owned gmail adapters can be shared", domain.ErrValidation)
	}
	validTargets, err := s.validWorkspaceTargets(ctx, systemWorkspace.TenantID, workspaceEnvironment(systemWorkspace), workspaceIDs)
	if err != nil {
		return err
	}
	current, err := s.adapterGrants.ListAdapterWorkspaceGrants(ctx, adapterID)
	if err != nil {
		return err
	}
	revoked := subtractUUIDs(current, validTargets)
	if len(revoked) == 0 {
		return s.adapterGrants.ReplaceAdapterWorkspaceGrants(ctx, adapterID, validTargets)
	}
	inUse, err := s.templateTypeUsages.ListWorkspacesUsingAdapter(ctx, adapterID, revoked)
	if err != nil {
		return err
	}
	if len(inUse) > 0 {
		return domain.ErrSharedGrantInUse
	}
	return s.adapterGrants.ReplaceAdapterWorkspaceGrants(ctx, adapterID, validTargets)
}

// ReplaceIdentityWorkspaceAccess replaces the workspace grant set for a shared email identity.
func (s *AdapterAccessService) ReplaceIdentityWorkspaceAccess(ctx context.Context, systemWorkspace *domain.Workspace, adapterID, identityID uuid.UUID, workspaceIDs []uuid.UUID) error {
	if systemWorkspace == nil || !systemWorkspace.IsSystem {
		return fmt.Errorf("%w: identity grants can only be managed from _system", domain.ErrForbidden)
	}
	adapter, err := s.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return err
	}
	if adapter.WorkspaceID == nil || *adapter.WorkspaceID != systemWorkspace.ID || !usesIdentityGrants(adapter.AdapterType) {
		return fmt.Errorf("%w: only system-owned identity-scoped adapters can share email identities", domain.ErrValidation)
	}
	identity, err := s.identityStore.GetByID(ctx, identityID)
	if err != nil {
		return err
	}
	if identity.AdapterID != adapterID || identity.IdentityType != domain.IdentityTypeEmail {
		return fmt.Errorf("%w: only email identities can be shared", domain.ErrValidation)
	}
	validTargets, err := s.validWorkspaceTargets(ctx, systemWorkspace.TenantID, workspaceEnvironment(systemWorkspace), workspaceIDs)
	if err != nil {
		return err
	}
	current, err := s.identityGrants.ListIdentityWorkspaceGrants(ctx, identityID)
	if err != nil {
		return err
	}
	revoked := subtractUUIDs(current, validTargets)
	if len(revoked) == 0 {
		return s.identityGrants.ReplaceIdentityWorkspaceGrants(ctx, identityID, validTargets)
	}
	inUse, err := s.templateTypeUsages.ListWorkspacesUsingSenderIdentity(ctx, identityID, revoked)
	if err != nil {
		return err
	}
	if len(inUse) > 0 {
		return domain.ErrSharedGrantInUse
	}
	return s.identityGrants.ReplaceIdentityWorkspaceGrants(ctx, identityID, validTargets)
}

// ListAdapterWorkspaceAccess returns tenant workspaces and whether the adapter is granted to each one.
func (s *AdapterAccessService) ListAdapterWorkspaceAccess(ctx context.Context, systemWorkspace *domain.Workspace, adapterID uuid.UUID) ([]WorkspaceAccessGrant, error) {
	if systemWorkspace == nil || !systemWorkspace.IsSystem {
		return nil, fmt.Errorf("%w: adapter grants can only be managed from _system", domain.ErrForbidden)
	}
	adapter, err := s.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	if adapter.WorkspaceID == nil || *adapter.WorkspaceID != systemWorkspace.ID || adapter.AdapterType != domain.AdapterTypeGmail {
		return nil, fmt.Errorf("%w: only system-owned gmail adapters can be shared", domain.ErrValidation)
	}
	return s.listWorkspaceAccess(ctx, systemWorkspace.TenantID, workspaceEnvironment(systemWorkspace), func() ([]uuid.UUID, error) {
		return s.adapterGrants.ListAdapterWorkspaceGrants(ctx, adapterID)
	})
}

// ListIdentityWorkspaceAccess returns tenant workspaces and whether the email identity is granted to each one.
func (s *AdapterAccessService) ListIdentityWorkspaceAccess(ctx context.Context, systemWorkspace *domain.Workspace, adapterID, identityID uuid.UUID) ([]WorkspaceAccessGrant, error) {
	if systemWorkspace == nil || !systemWorkspace.IsSystem {
		return nil, fmt.Errorf("%w: identity grants can only be managed from _system", domain.ErrForbidden)
	}
	adapter, err := s.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	if adapter.WorkspaceID == nil || *adapter.WorkspaceID != systemWorkspace.ID || !usesIdentityGrants(adapter.AdapterType) {
		return nil, fmt.Errorf("%w: only system-owned identity-scoped adapters can share email identities", domain.ErrValidation)
	}
	identity, err := s.identityStore.GetByID(ctx, identityID)
	if err != nil {
		return nil, err
	}
	if identity.AdapterID != adapterID || identity.IdentityType != domain.IdentityTypeEmail {
		return nil, fmt.Errorf("%w: only email identities can be shared", domain.ErrValidation)
	}
	return s.listWorkspaceAccess(ctx, systemWorkspace.TenantID, workspaceEnvironment(systemWorkspace), func() ([]uuid.UUID, error) {
		return s.identityGrants.ListIdentityWorkspaceGrants(ctx, identityID)
	})
}

func (s *AdapterAccessService) validateSenderIdentity(ctx context.Context, adapter *domain.Adapter, senderIdentityID *uuid.UUID, extra func(identity *domain.AdapterIdentity) error) error {
	if senderIdentityID == nil {
		return nil
	}
	identity, err := s.identityStore.GetByID(ctx, *senderIdentityID)
	if err != nil {
		return err
	}
	if identity.AdapterID != adapter.ID {
		return domain.ErrSenderIdentityAccessDenied
	}
	if identity.IdentityType != domain.IdentityTypeEmail {
		return domain.ErrSenderIdentityAccessDenied
	}
	if extra != nil {
		if err := extra(identity); err != nil {
			return err
		}
	}
	return nil
}

func usesIdentityGrants(adapterType domain.AdapterType) bool {
	return adapterType == domain.AdapterTypeSES || adapterType == domain.AdapterTypeSMTP
}

func workspaceEnvironment(workspace *domain.Workspace) domain.Environment {
	if workspace != nil && workspace.Environment.Valid() {
		return workspace.Environment
	}
	return domain.EnvironmentProd
}

func (s *AdapterAccessService) validWorkspaceTargets(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, requested []uuid.UUID) ([]uuid.UUID, error) {
	if len(requested) == 0 {
		return nil, nil
	}
	workspaces, _, err := s.workspaceStore.ListByTenant(ctx, tenantID, environment, port.ListOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}
	allowed := make(map[uuid.UUID]*domain.Workspace, len(workspaces))
	for _, ws := range workspaces {
		if ws.IsSystem {
			continue
		}
		allowed[ws.ID] = ws
	}
	result := make([]uuid.UUID, 0, len(requested))
	seen := make(map[uuid.UUID]struct{}, len(requested))
	for _, id := range requested {
		if _, ok := seen[id]; ok {
			continue
		}
		if _, ok := allowed[id]; !ok {
			return nil, fmt.Errorf("%w: invalid workspace target %s", domain.ErrValidation, id)
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func (s *AdapterAccessService) listWorkspaceAccess(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, loadGrants func() ([]uuid.UUID, error)) ([]WorkspaceAccessGrant, error) {
	workspaces, _, err := s.workspaceStore.ListByTenant(ctx, tenantID, environment, port.ListOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}
	grants, err := loadGrants()
	if err != nil {
		return nil, err
	}
	granted := make(map[uuid.UUID]struct{}, len(grants))
	for _, id := range grants {
		granted[id] = struct{}{}
	}
	result := make([]WorkspaceAccessGrant, 0, len(workspaces))
	for _, ws := range workspaces {
		if ws.IsSystem {
			continue
		}
		_, ok := granted[ws.ID]
		result = append(result, WorkspaceAccessGrant{Workspace: ws, Granted: ok})
	}
	slices.SortFunc(result, func(a, b WorkspaceAccessGrant) int {
		if a.Workspace.Name == b.Workspace.Name {
			switch {
			case a.Workspace.Code < b.Workspace.Code:
				return -1
			case a.Workspace.Code > b.Workspace.Code:
				return 1
			default:
				return 0
			}
		}
		switch {
		case a.Workspace.Name < b.Workspace.Name:
			return -1
		case a.Workspace.Name > b.Workspace.Name:
			return 1
		default:
			return 0
		}
	})
	return result, nil
}

func subtractUUIDs(current, desired []uuid.UUID) []uuid.UUID {
	if len(current) == 0 {
		return nil
	}
	desiredSet := make(map[uuid.UUID]struct{}, len(desired))
	for _, id := range desired {
		desiredSet[id] = struct{}{}
	}
	result := make([]uuid.UUID, 0, len(current))
	for _, id := range current {
		if _, ok := desiredSet[id]; ok {
			continue
		}
		result = append(result, id)
	}
	return result
}

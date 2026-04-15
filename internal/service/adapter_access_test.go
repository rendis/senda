package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

type mockAdapterGrantStore struct {
	listAdapterWorkspaceGrantsFn    func(ctx context.Context, adapterID uuid.UUID) ([]uuid.UUID, error)
	replaceAdapterWorkspaceGrantsFn func(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) error
	hasAdapterWorkspaceGrantFn      func(ctx context.Context, adapterID, workspaceID uuid.UUID) (bool, error)
	listVisibleAdaptersFn           func(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error)
}

func (m *mockAdapterGrantStore) ListAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID) ([]uuid.UUID, error) {
	if m.listAdapterWorkspaceGrantsFn != nil {
		return m.listAdapterWorkspaceGrantsFn(ctx, adapterID)
	}
	return nil, nil
}

func (m *mockAdapterGrantStore) ReplaceAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) error {
	if m.replaceAdapterWorkspaceGrantsFn != nil {
		return m.replaceAdapterWorkspaceGrantsFn(ctx, adapterID, workspaceIDs)
	}
	return nil
}

func (m *mockAdapterGrantStore) HasAdapterWorkspaceGrant(ctx context.Context, adapterID, workspaceID uuid.UUID) (bool, error) {
	if m.hasAdapterWorkspaceGrantFn != nil {
		return m.hasAdapterWorkspaceGrantFn(ctx, adapterID, workspaceID)
	}
	return false, nil
}

func (m *mockAdapterGrantStore) ListVisibleAdaptersForWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	if m.listVisibleAdaptersFn != nil {
		return m.listVisibleAdaptersFn(ctx, workspaceID, opts)
	}
	return &port.PageResult[domain.Adapter]{Items: []*domain.Adapter{}}, nil
}

type mockIdentityGrantStore struct {
	listIdentityWorkspaceGrantsFn    func(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error)
	replaceIdentityWorkspaceGrantsFn func(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) error
	hasIdentityWorkspaceGrantFn      func(ctx context.Context, identityID, workspaceID uuid.UUID) (bool, error)
	listGrantedIdentitiesFn          func(ctx context.Context, adapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error)
}

func (m *mockIdentityGrantStore) ListIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error) {
	if m.listIdentityWorkspaceGrantsFn != nil {
		return m.listIdentityWorkspaceGrantsFn(ctx, identityID)
	}
	return nil, nil
}

func (m *mockIdentityGrantStore) ReplaceIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) error {
	if m.replaceIdentityWorkspaceGrantsFn != nil {
		return m.replaceIdentityWorkspaceGrantsFn(ctx, identityID, workspaceIDs)
	}
	return nil
}

func (m *mockIdentityGrantStore) HasIdentityWorkspaceGrant(ctx context.Context, identityID, workspaceID uuid.UUID) (bool, error) {
	if m.hasIdentityWorkspaceGrantFn != nil {
		return m.hasIdentityWorkspaceGrantFn(ctx, identityID, workspaceID)
	}
	return false, nil
}

func (m *mockIdentityGrantStore) ListGrantedIdentitiesForWorkspace(ctx context.Context, adapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	if m.listGrantedIdentitiesFn != nil {
		return m.listGrantedIdentitiesFn(ctx, adapterID, workspaceID)
	}
	return nil, nil
}

type mockTemplateTypeUsageStore struct {
	listWorkspacesUsingAdapterFn        func(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error)
	listWorkspacesUsingSenderIdentityFn func(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error)
}

func (m *mockTemplateTypeUsageStore) ListWorkspacesUsingAdapter(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error) {
	if m.listWorkspacesUsingAdapterFn != nil {
		return m.listWorkspacesUsingAdapterFn(ctx, adapterID, workspaceIDs)
	}
	return nil, nil
}

func (m *mockTemplateTypeUsageStore) ListWorkspacesUsingSenderIdentity(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error) {
	if m.listWorkspacesUsingSenderIdentityFn != nil {
		return m.listWorkspacesUsingSenderIdentityFn(ctx, identityID, workspaceIDs)
	}
	return nil, nil
}

func TestAdapterAccessService_ValidateSelection_SharedSESRequiresGrantedEmailIdentity(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{ID: systemWorkspaceID, TenantID: uuid.Must(uuid.NewV7()), Code: "_system", IsSystem: true}
	workspace := &domain.Workspace{ID: workspaceID, TenantID: systemWorkspace.TenantID, Code: "default"}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeSES,
				Name:        "SES Shared",
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != identityID {
				return nil, domain.ErrIdentityNotFound
			}
			return &domain.AdapterIdentity{
				ID:           identityID,
				AdapterID:    adapterID,
				Identity:     "a@example.dev",
				IdentityType: domain.IdentityTypeEmail,
				Status:       domain.IdentityStatusVerified,
			}, nil
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			switch id {
			case systemWorkspaceID:
				return systemWorkspace, nil
			case workspaceID:
				return workspace, nil
			default:
				return nil, domain.ErrNotFound
			}
		},
	}

	identityGrants := &mockIdentityGrantStore{
		hasIdentityWorkspaceGrantFn: func(_ context.Context, identity uuid.UUID, wsID uuid.UUID) (bool, error) {
			return identity == identityID && wsID == workspaceID, nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		identityStore,
		wsStore,
		&mockAdapterGrantStore{},
		identityGrants,
		&mockTemplateTypeUsageStore{},
	)

	err := svc.ValidateTemplateTypeSelection(context.Background(), workspace, &adapterID, nil)
	if !errors.Is(err, domain.ErrSenderIdentityRequired) {
		t.Fatalf("expected ErrSenderIdentityRequired, got %v", err)
	}

	err = svc.ValidateTemplateTypeSelection(context.Background(), workspace, &adapterID, &identityID)
	if err != nil {
		t.Fatalf("expected shared SES selection to pass with granted email identity, got %v", err)
	}
}

func TestAdapterAccessService_ValidateSelection_SharedGmailRequiresAdapterGrant(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{ID: systemWorkspaceID, TenantID: uuid.Must(uuid.NewV7()), Code: "_system", IsSystem: true}
	workspace := &domain.Workspace{ID: workspaceID, TenantID: systemWorkspace.TenantID, Code: "default"}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeGmail,
				Name:        "Gmail Shared",
			}, nil
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			switch id {
			case systemWorkspaceID:
				return systemWorkspace, nil
			case workspaceID:
				return workspace, nil
			default:
				return nil, domain.ErrNotFound
			}
		},
	}

	grants := &mockAdapterGrantStore{
		hasAdapterWorkspaceGrantFn: func(_ context.Context, id uuid.UUID, wsID uuid.UUID) (bool, error) {
			return id == adapterID && wsID == workspaceID, nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		&mockAdapterIdentityStoreSend{},
		wsStore,
		grants,
		&mockIdentityGrantStore{},
		&mockTemplateTypeUsageStore{},
	)

	if err := svc.ValidateTemplateTypeSelection(context.Background(), workspace, &adapterID, nil); err != nil {
		t.Fatalf("expected shared gmail selection to pass with adapter grant, got %v", err)
	}
}

func TestAdapterAccessService_ListIdentitiesForWorkspace_FiltersSharedSESIdentities(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{ID: systemWorkspaceID, TenantID: uuid.Must(uuid.NewV7()), Code: "_system", IsSystem: true}
	workspace := &domain.Workspace{ID: workspaceID, TenantID: systemWorkspace.TenantID, Code: "default"}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeSES,
				Name:        "SES Shared",
			}, nil
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			switch id {
			case systemWorkspaceID:
				return systemWorkspace, nil
			case workspaceID:
				return workspace, nil
			default:
				return nil, domain.ErrNotFound
			}
		},
	}

	identityGrants := &mockIdentityGrantStore{
		listGrantedIdentitiesFn: func(_ context.Context, adapter uuid.UUID, wsID uuid.UUID) ([]*domain.AdapterIdentity, error) {
			if adapter != adapterID || wsID != workspaceID {
				return nil, errors.New("unexpected grant lookup")
			}
			return []*domain.AdapterIdentity{
				{ID: uuid.Must(uuid.NewV7()), AdapterID: adapterID, Identity: "a@example.dev", IdentityType: domain.IdentityTypeEmail},
			}, nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		&mockAdapterIdentityStoreSend{},
		wsStore,
		&mockAdapterGrantStore{},
		identityGrants,
		&mockTemplateTypeUsageStore{},
	)

	identities, err := svc.ListIdentitiesForWorkspace(context.Background(), workspace, adapterID)
	if err != nil {
		t.Fatalf("unexpected error listing shared SES identities: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("expected 1 granted identity, got %d", len(identities))
	}
	if identities[0].Identity != "a@example.dev" {
		t.Fatalf("expected granted identity a@example.dev, got %q", identities[0].Identity)
	}
}

func TestAdapterAccessService_ReplaceIdentityWorkspaceAccess_BlocksRevokeWhenInUse(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{ID: systemWorkspaceID, TenantID: uuid.Must(uuid.NewV7()), Code: "_system", IsSystem: true}
	workspace := &domain.Workspace{ID: workspaceID, TenantID: systemWorkspace.TenantID, Code: "default"}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeSES,
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != identityID {
				return nil, domain.ErrIdentityNotFound
			}
			return &domain.AdapterIdentity{
				ID:           identityID,
				AdapterID:    adapterID,
				Identity:     "a@example.dev",
				IdentityType: domain.IdentityTypeEmail,
			}, nil
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		listByTenantFn: func(_ context.Context, tenantID uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			if tenantID != systemWorkspace.TenantID {
				return nil, "", errors.New("unexpected tenant lookup")
			}
			return []*domain.Workspace{systemWorkspace, workspace}, "", nil
		},
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			switch id {
			case systemWorkspaceID:
				return systemWorkspace, nil
			case workspaceID:
				return workspace, nil
			default:
				return nil, domain.ErrNotFound
			}
		},
	}

	identityGrants := &mockIdentityGrantStore{
		listIdentityWorkspaceGrantsFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			if id != identityID {
				return nil, errors.New("unexpected identity grant lookup")
			}
			return []uuid.UUID{workspaceID}, nil
		},
	}

	usageStore := &mockTemplateTypeUsageStore{
		listWorkspacesUsingSenderIdentityFn: func(_ context.Context, id uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error) {
			if id != identityID || len(workspaceIDs) != 1 || workspaceIDs[0] != workspaceID {
				return nil, errors.New("unexpected usage lookup")
			}
			return []uuid.UUID{workspaceID}, nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		identityStore,
		wsStore,
		&mockAdapterGrantStore{},
		identityGrants,
		usageStore,
	)

	err := svc.ReplaceIdentityWorkspaceAccess(context.Background(), systemWorkspace, adapterID, identityID, nil)
	if !errors.Is(err, domain.ErrSharedGrantInUse) {
		t.Fatalf("expected ErrSharedGrantInUse, got %v", err)
	}
}

func TestAdapterAccessService_ReplaceIdentityWorkspaceAccess_RejectsDomainIdentity(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{ID: systemWorkspaceID, TenantID: uuid.Must(uuid.NewV7()), Code: "_system", IsSystem: true}
	workspace := &domain.Workspace{ID: workspaceID, TenantID: systemWorkspace.TenantID, Code: "default"}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeSES,
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != identityID {
				return nil, domain.ErrIdentityNotFound
			}
			return &domain.AdapterIdentity{
				ID:           identityID,
				AdapterID:    adapterID,
				Identity:     "example.dev",
				IdentityType: domain.IdentityTypeDomain,
			}, nil
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		listByTenantFn: func(_ context.Context, tenantID uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			if tenantID != systemWorkspace.TenantID {
				return nil, "", errors.New("unexpected tenant lookup")
			}
			return []*domain.Workspace{systemWorkspace, workspace}, "", nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		identityStore,
		wsStore,
		&mockAdapterGrantStore{},
		&mockIdentityGrantStore{},
		&mockTemplateTypeUsageStore{},
	)

	err := svc.ReplaceIdentityWorkspaceAccess(context.Background(), systemWorkspace, adapterID, identityID, []uuid.UUID{workspaceID})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation for domain identity sharing, got %v", err)
	}
}

func TestAdapterAccessService_ReplaceAdapterWorkspaceAccess_UsesSystemWorkspaceEnvironment(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{
		ID:          systemWorkspaceID,
		TenantID:    uuid.Must(uuid.NewV7()),
		Code:        "_system",
		IsSystem:    true,
		Environment: domain.EnvironmentTest,
	}
	workspace := &domain.Workspace{
		ID:          workspaceID,
		TenantID:    systemWorkspace.TenantID,
		Code:        "default",
		Environment: domain.EnvironmentTest,
	}

	var listedEnvironment domain.Environment
	var replacedTargets []uuid.UUID

	wsStore := &mockWorkspaceStoreSend{
		listByTenantFn: func(_ context.Context, tenantID uuid.UUID, environment domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			if tenantID != systemWorkspace.TenantID {
				return nil, "", errors.New("unexpected tenant lookup")
			}
			listedEnvironment = environment
			return []*domain.Workspace{systemWorkspace, workspace}, "", nil
		},
	}
	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeGmail,
			}, nil
		},
	}
	adapterGrants := &mockAdapterGrantStore{
		replaceAdapterWorkspaceGrantsFn: func(_ context.Context, id uuid.UUID, workspaceIDs []uuid.UUID) error {
			if id != adapterID {
				return errors.New("unexpected adapter id")
			}
			replacedTargets = append([]uuid.UUID(nil), workspaceIDs...)
			return nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		&mockAdapterIdentityStoreSend{},
		wsStore,
		adapterGrants,
		&mockIdentityGrantStore{},
		&mockTemplateTypeUsageStore{},
	)

	if err := svc.ReplaceAdapterWorkspaceAccess(context.Background(), systemWorkspace, adapterID, []uuid.UUID{workspaceID}); err != nil {
		t.Fatalf("expected replace grants to succeed, got %v", err)
	}
	if listedEnvironment != domain.EnvironmentTest {
		t.Fatalf("expected workspace validation in test environment, got %s", listedEnvironment)
	}
	if len(replacedTargets) != 1 || replacedTargets[0] != workspaceID {
		t.Fatalf("expected replace grants with workspace %s, got %v", workspaceID, replacedTargets)
	}
}

func TestAdapterAccessService_ListIdentityWorkspaceAccess_UsesSystemWorkspaceEnvironment(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{
		ID:          systemWorkspaceID,
		TenantID:    uuid.Must(uuid.NewV7()),
		Code:        "_system",
		IsSystem:    true,
		Environment: domain.EnvironmentTest,
	}
	workspace := &domain.Workspace{
		ID:          workspaceID,
		TenantID:    systemWorkspace.TenantID,
		Code:        "default",
		Name:        "Default",
		Environment: domain.EnvironmentTest,
	}

	var listedEnvironment domain.Environment

	wsStore := &mockWorkspaceStoreSend{
		listByTenantFn: func(_ context.Context, tenantID uuid.UUID, environment domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			if tenantID != systemWorkspace.TenantID {
				return nil, "", errors.New("unexpected tenant lookup")
			}
			listedEnvironment = environment
			return []*domain.Workspace{systemWorkspace, workspace}, "", nil
		},
	}
	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeSES,
			}, nil
		},
	}
	identityStore := &mockAdapterIdentityStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != identityID {
				return nil, domain.ErrIdentityNotFound
			}
			return &domain.AdapterIdentity{
				ID:           identityID,
				AdapterID:    adapterID,
				Identity:     "a@example.dev",
				IdentityType: domain.IdentityTypeEmail,
			}, nil
		},
	}
	identityGrants := &mockIdentityGrantStore{
		listIdentityWorkspaceGrantsFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			if id != identityID {
				return nil, errors.New("unexpected identity id")
			}
			return []uuid.UUID{workspaceID}, nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		identityStore,
		wsStore,
		&mockAdapterGrantStore{},
		identityGrants,
		&mockTemplateTypeUsageStore{},
	)

	grants, err := svc.ListIdentityWorkspaceAccess(context.Background(), systemWorkspace, adapterID, identityID)
	if err != nil {
		t.Fatalf("expected list grants to succeed, got %v", err)
	}
	if listedEnvironment != domain.EnvironmentTest {
		t.Fatalf("expected grant listing in test environment, got %s", listedEnvironment)
	}
	if len(grants) != 1 || grants[0].Workspace.ID != workspaceID || !grants[0].Granted {
		t.Fatalf("expected granted workspace %s, got %+v", workspaceID, grants)
	}
}

func TestAdapterAccessService_ReplaceIdentityWorkspaceAccess_UsesSetBasedUsageCheck(t *testing.T) {
	systemWorkspaceID := uuid.Must(uuid.NewV7())
	workspaceAID := uuid.Must(uuid.NewV7())
	workspaceBID := uuid.Must(uuid.NewV7())
	workspaceCID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())

	systemWorkspace := &domain.Workspace{ID: systemWorkspaceID, TenantID: uuid.Must(uuid.NewV7()), Code: "_system", IsSystem: true}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWorkspaceID,
				AdapterType: domain.AdapterTypeSES,
				Name:        "SES Shared",
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != identityID {
				return nil, domain.ErrIdentityNotFound
			}
			return &domain.AdapterIdentity{
				ID:           identityID,
				AdapterID:    adapterID,
				Identity:     "a@example.dev",
				IdentityType: domain.IdentityTypeEmail,
				Status:       domain.IdentityStatusVerified,
			}, nil
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			if id == systemWorkspaceID {
				return systemWorkspace, nil
			}
			return nil, domain.ErrNotFound
		},
		listByTenantFn: func(_ context.Context, tenantID uuid.UUID, environment domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return []*domain.Workspace{
				{ID: workspaceAID, TenantID: tenantID, Code: "alpha", Name: "Alpha", Environment: environment},
				{ID: workspaceBID, TenantID: tenantID, Code: "beta", Name: "Beta", Environment: environment},
				{ID: workspaceCID, TenantID: tenantID, Code: "gamma", Name: "Gamma", Environment: environment},
			}, "", nil
		},
	}

	var usageCalls int
	var requested []uuid.UUID
	usageStore := &mockTemplateTypeUsageStore{
		listWorkspacesUsingSenderIdentityFn: func(_ context.Context, id uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error) {
			usageCalls++
			if id != identityID {
				t.Fatalf("expected identity %s, got %s", identityID, id)
			}
			requested = append([]uuid.UUID(nil), workspaceIDs...)
			return []uuid.UUID{workspaceCID}, nil
		},
	}

	grantStore := &mockIdentityGrantStore{
		listIdentityWorkspaceGrantsFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			if id != identityID {
				t.Fatalf("expected identity %s, got %s", identityID, id)
			}
			return []uuid.UUID{workspaceAID, workspaceBID, workspaceCID}, nil
		},
	}

	svc := service.NewAdapterAccessService(
		adapterStore,
		identityStore,
		wsStore,
		&mockAdapterGrantStore{},
		grantStore,
		usageStore,
	)

	err := svc.ReplaceIdentityWorkspaceAccess(context.Background(), systemWorkspace, adapterID, identityID, []uuid.UUID{workspaceAID})
	if !errors.Is(err, domain.ErrSharedGrantInUse) {
		t.Fatalf("expected ErrSharedGrantInUse, got %v", err)
	}
	if usageCalls != 1 {
		t.Fatalf("expected exactly one usage lookup, got %d", usageCalls)
	}
	if len(requested) != 2 || requested[0] != workspaceBID || requested[1] != workspaceCID {
		t.Fatalf("expected revoked workspaces [B C], got %v", requested)
	}
}

func BenchmarkAdapterAccessService_ReplaceIdentityWorkspaceAccess_SetBased(b *testing.B) {
	b.ReportAllocs()

	const workspaceCount = 256

	systemWorkspaceID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())
	tenantID := uuid.Must(uuid.NewV7())
	systemWorkspace := &domain.Workspace{ID: systemWorkspaceID, TenantID: tenantID, Code: "_system", IsSystem: true}

	current := make([]uuid.UUID, 0, workspaceCount)
	targets := make([]uuid.UUID, 0, workspaceCount/2)
	workspaces := make([]*domain.Workspace, 0, workspaceCount)
	for i := 0; i < workspaceCount; i++ {
		wsID := uuid.Must(uuid.NewV7())
		current = append(current, wsID)
		workspaces = append(workspaces, &domain.Workspace{
			ID:          wsID,
			TenantID:    tenantID,
			Code:        fmt.Sprintf("ws-%03d", i),
			Name:        fmt.Sprintf("Workspace %03d", i),
			Environment: domain.EnvironmentProd,
		})
		if i%2 == 0 {
			targets = append(targets, wsID)
		}
	}

	svc := service.NewAdapterAccessService(
		&mockAdapterStoreSend{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
				if id != adapterID {
					return nil, domain.ErrNotFound
				}
				return &domain.Adapter{
					ID:          adapterID,
					WorkspaceID: &systemWorkspaceID,
					AdapterType: domain.AdapterTypeSES,
					Name:        "SES Shared",
				}, nil
			},
		},
		&mockAdapterIdentityStoreSend{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
				if id != identityID {
					return nil, domain.ErrIdentityNotFound
				}
				return &domain.AdapterIdentity{
					ID:           identityID,
					AdapterID:    adapterID,
					Identity:     "a@example.dev",
					IdentityType: domain.IdentityTypeEmail,
					Status:       domain.IdentityStatusVerified,
				}, nil
			},
		},
		&mockWorkspaceStoreSend{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
				if id != systemWorkspaceID {
					return nil, domain.ErrNotFound
				}
				return systemWorkspace, nil
			},
			listByTenantFn: func(_ context.Context, gotTenantID uuid.UUID, environment domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
				if gotTenantID != tenantID {
					return nil, "", domain.ErrNotFound
				}
				if environment != domain.EnvironmentProd {
					return nil, "", domain.ErrNotFound
				}
				return workspaces, "", nil
			},
		},
		&mockAdapterGrantStore{},
		&mockIdentityGrantStore{
			listIdentityWorkspaceGrantsFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
				if id != identityID {
					return nil, domain.ErrIdentityNotFound
				}
				return append([]uuid.UUID(nil), current...), nil
			},
			replaceIdentityWorkspaceGrantsFn: func(_ context.Context, gotIdentityID uuid.UUID, workspaceIDs []uuid.UUID) error {
				if gotIdentityID != identityID {
					return domain.ErrIdentityNotFound
				}
				if len(workspaceIDs) != len(targets) {
					return fmt.Errorf("unexpected replacement size %d", len(workspaceIDs))
				}
				return nil
			},
		},
		&mockTemplateTypeUsageStore{
			listWorkspacesUsingSenderIdentityFn: func(_ context.Context, gotIdentityID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error) {
				if gotIdentityID != identityID {
					return nil, domain.ErrIdentityNotFound
				}
				return nil, nil
			},
		},
	)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := svc.ReplaceIdentityWorkspaceAccess(ctx, systemWorkspace, adapterID, identityID, targets); err != nil {
			b.Fatalf("ReplaceIdentityWorkspaceAccess() error: %v", err)
		}
	}
}

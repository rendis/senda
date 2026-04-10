package service_test

import (
	"context"
	"errors"
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
	countTypesUsingAdapterFn        func(ctx context.Context, adapterID uuid.UUID, workspaceID *uuid.UUID) (int, error)
	countTypesUsingSenderIdentityFn func(ctx context.Context, identityID uuid.UUID, workspaceID *uuid.UUID) (int, error)
}

func (m *mockTemplateTypeUsageStore) CountTypesUsingAdapter(ctx context.Context, adapterID uuid.UUID, workspaceID *uuid.UUID) (int, error) {
	if m.countTypesUsingAdapterFn != nil {
		return m.countTypesUsingAdapterFn(ctx, adapterID, workspaceID)
	}
	return 0, nil
}

func (m *mockTemplateTypeUsageStore) CountTypesUsingSenderIdentity(ctx context.Context, identityID uuid.UUID, workspaceID *uuid.UUID) (int, error) {
	if m.countTypesUsingSenderIdentityFn != nil {
		return m.countTypesUsingSenderIdentityFn(ctx, identityID, workspaceID)
	}
	return 0, nil
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
		countTypesUsingSenderIdentityFn: func(_ context.Context, id uuid.UUID, wsID *uuid.UUID) (int, error) {
			if id != identityID || wsID == nil || *wsID != workspaceID {
				return 0, errors.New("unexpected usage lookup")
			}
			return 1, nil
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

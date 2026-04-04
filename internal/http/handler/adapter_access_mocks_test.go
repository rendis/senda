package handler_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

type mockAuditLogStoreHandler struct {
	appendFn func(ctx context.Context, entry *domain.AuditLog) error
	entries  []*domain.AuditLog
}

func (m *mockAuditLogStoreHandler) Append(ctx context.Context, entry *domain.AuditLog) error {
	if m.appendFn != nil {
		return m.appendFn(ctx, entry)
	}
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockAuditLogStoreHandler) Query(_ context.Context, _ port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
	return &port.PageResult[domain.AuditLog]{Items: []*domain.AuditLog{}}, nil
}

type mockAdapterGrantStoreHandler struct {
	listAdapterWorkspaceGrantsFn    func(ctx context.Context, adapterID uuid.UUID) ([]uuid.UUID, error)
	replaceAdapterWorkspaceGrantsFn func(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) error
	hasAdapterWorkspaceGrantFn      func(ctx context.Context, adapterID, workspaceID uuid.UUID) (bool, error)
	listVisibleAdaptersFn           func(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error)
}

func (m *mockAdapterGrantStoreHandler) ListAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID) ([]uuid.UUID, error) {
	if m.listAdapterWorkspaceGrantsFn != nil {
		return m.listAdapterWorkspaceGrantsFn(ctx, adapterID)
	}
	return nil, nil
}

func (m *mockAdapterGrantStoreHandler) ReplaceAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) error {
	if m.replaceAdapterWorkspaceGrantsFn != nil {
		return m.replaceAdapterWorkspaceGrantsFn(ctx, adapterID, workspaceIDs)
	}
	return nil
}

func (m *mockAdapterGrantStoreHandler) HasAdapterWorkspaceGrant(ctx context.Context, adapterID, workspaceID uuid.UUID) (bool, error) {
	if m.hasAdapterWorkspaceGrantFn != nil {
		return m.hasAdapterWorkspaceGrantFn(ctx, adapterID, workspaceID)
	}
	return false, nil
}

func (m *mockAdapterGrantStoreHandler) ListVisibleAdaptersForWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	if m.listVisibleAdaptersFn != nil {
		return m.listVisibleAdaptersFn(ctx, workspaceID, opts)
	}
	return &port.PageResult[domain.Adapter]{Items: []*domain.Adapter{}}, nil
}

type mockIdentityGrantStoreHandler struct {
	listIdentityWorkspaceGrantsFn    func(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error)
	replaceIdentityWorkspaceGrantsFn func(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) error
	hasIdentityWorkspaceGrantFn      func(ctx context.Context, identityID, workspaceID uuid.UUID) (bool, error)
	listGrantedIdentitiesFn          func(ctx context.Context, adapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error)
}

func (m *mockIdentityGrantStoreHandler) ListIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error) {
	if m.listIdentityWorkspaceGrantsFn != nil {
		return m.listIdentityWorkspaceGrantsFn(ctx, identityID)
	}
	return nil, nil
}

func (m *mockIdentityGrantStoreHandler) ReplaceIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) error {
	if m.replaceIdentityWorkspaceGrantsFn != nil {
		return m.replaceIdentityWorkspaceGrantsFn(ctx, identityID, workspaceIDs)
	}
	return nil
}

func (m *mockIdentityGrantStoreHandler) HasIdentityWorkspaceGrant(ctx context.Context, identityID, workspaceID uuid.UUID) (bool, error) {
	if m.hasIdentityWorkspaceGrantFn != nil {
		return m.hasIdentityWorkspaceGrantFn(ctx, identityID, workspaceID)
	}
	return false, nil
}

func (m *mockIdentityGrantStoreHandler) ListGrantedIdentitiesForWorkspace(ctx context.Context, adapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	if m.listGrantedIdentitiesFn != nil {
		return m.listGrantedIdentitiesFn(ctx, adapterID, workspaceID)
	}
	return nil, nil
}

type mockTemplateTypeUsageStoreHandler struct {
	countTypesUsingAdapterFn        func(ctx context.Context, adapterID uuid.UUID, workspaceID *uuid.UUID) (int, error)
	countTypesUsingSenderIdentityFn func(ctx context.Context, identityID uuid.UUID, workspaceID *uuid.UUID) (int, error)
}

func (m *mockTemplateTypeUsageStoreHandler) CountTypesUsingAdapter(ctx context.Context, adapterID uuid.UUID, workspaceID *uuid.UUID) (int, error) {
	if m.countTypesUsingAdapterFn != nil {
		return m.countTypesUsingAdapterFn(ctx, adapterID, workspaceID)
	}
	return 0, nil
}

func (m *mockTemplateTypeUsageStoreHandler) CountTypesUsingSenderIdentity(ctx context.Context, identityID uuid.UUID, workspaceID *uuid.UUID) (int, error) {
	if m.countTypesUsingSenderIdentityFn != nil {
		return m.countTypesUsingSenderIdentityFn(ctx, identityID, workspaceID)
	}
	return 0, nil
}

package resolution_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/resolution"
)

// --- Mock DomainStore ---

type mockDomainStore struct {
	listInChain func(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error)
}

func (m *mockDomainStore) Create(_ context.Context, _ *domain.Domain) error { return nil }
func (m *mockDomainStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Domain, error) {
	return nil, nil
}
func (m *mockDomainStore) Update(_ context.Context, _ *domain.Domain) error { return nil }
func (m *mockDomainStore) SoftDelete(_ context.Context, _ uuid.UUID) error  { return nil }
func (m *mockDomainStore) ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error) {
	return m.listInChain(ctx, scopes)
}
func (m *mockDomainStore) ListByWorkspace(_ context.Context, _ *uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.Domain], error) {
	return nil, nil
}
func (m *mockDomainStore) GetPendingVerifications(_ context.Context, _ int) ([]*domain.Domain, error) {
	return nil, nil
}

// --- DomainResolver Tests ---

func TestDomainResolver_VerifiedDomain(t *testing.T) {
	wsID := uuid.New()
	tenantID := uuid.New()
	sysID := uuid.New()

	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}
	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	wsStore := &mockWorkspaceStore{
		getByID:            func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
	}
	cache := newMockCache()
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			return []*domain.Domain{
				{ID: uuid.New(), DomainName: "example.com", Status: domain.DomainStatusVerified},
			}, nil
		},
	}

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache was populated
	key := fmt.Sprintf("domain_valid:%s:example.com", wsID.String())
	if _, getErr := cache.Get(context.Background(), key); getErr != nil {
		t.Error("expected domain_valid cache entry to be set")
	}
}

func TestDomainResolver_CachedDomain(t *testing.T) {
	wsID := uuid.New()
	cache := newMockCache()

	// Pre-populate cache
	key := fmt.Sprintf("domain_valid:%s:example.com", wsID.String())
	_ = cache.Set(context.Background(), key, []byte("1"), 10*time.Minute)

	storeCalled := false
	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			storeCalled = true
			return nil, errors.New("should not be called")
		},
	}

	// ChainResolver should also not be called, but we still need a valid one
	wsStore := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			storeCalled = true
			return nil, errors.New("should not be called")
		},
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, errors.New("should not be called")
		},
	}
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storeCalled {
		t.Error("store was called despite cache hit")
	}
}

func TestDomainResolver_DomainNotInChain(t *testing.T) {
	wsID := uuid.New()
	tenantID := uuid.New()
	sysID := uuid.New()

	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}
	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	wsStore := &mockWorkspaceStore{
		getByID:            func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
	}
	cache := newMockCache()
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			return []*domain.Domain{
				{ID: uuid.New(), DomainName: "other.com", Status: domain.DomainStatusVerified},
			}, nil
		},
	}

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrDomainNotVerified) {
		t.Errorf("expected ErrDomainNotVerified, got %v", err)
	}
}

func TestDomainResolver_PendingDomain(t *testing.T) {
	wsID := uuid.New()
	tenantID := uuid.New()
	sysID := uuid.New()

	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}
	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	wsStore := &mockWorkspaceStore{
		getByID:            func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
	}
	cache := newMockCache()
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			return []*domain.Domain{
				{ID: uuid.New(), DomainName: "example.com", Status: domain.DomainStatusPending},
			}, nil
		},
	}

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrDomainNotVerified) {
		t.Errorf("expected ErrDomainNotVerified, got %v", err)
	}
}

func TestDomainResolver_SoftDeletedDomain(t *testing.T) {
	wsID := uuid.New()
	tenantID := uuid.New()
	sysID := uuid.New()
	now := time.Now()

	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}
	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	wsStore := &mockWorkspaceStore{
		getByID:            func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
	}
	cache := newMockCache()
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			return []*domain.Domain{
				{ID: uuid.New(), DomainName: "example.com", Status: domain.DomainStatusVerified, DeletedAt: &now},
			}, nil
		},
	}

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrDomainNotVerified) {
		t.Errorf("expected ErrDomainNotVerified, got %v", err)
	}
}

func TestDomainResolver_InvalidEmail(t *testing.T) {
	wsID := uuid.New()
	cache := newMockCache()

	wsStore := &mockWorkspaceStore{
		getByID:            func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return nil, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return nil, nil },
	}
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			return nil, nil
		},
	}

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "invalid-no-at-sign")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrDomainNotVerified) {
		t.Errorf("expected ErrDomainNotVerified, got %v", err)
	}
}

func TestDomainResolver_ChainResolverError(t *testing.T) {
	wsID := uuid.New()
	cache := newMockCache()
	chainErr := errors.New("chain resolver failed")

	wsStore := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, chainErr
		},
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, nil
		},
	}
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			return nil, nil
		},
	}

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, chainErr) {
		t.Errorf("expected chain error to propagate, got %v", err)
	}
}

func TestDomainResolver_StoreError(t *testing.T) {
	wsID := uuid.New()
	tenantID := uuid.New()
	sysID := uuid.New()
	storeErr := errors.New("db connection failed")

	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}
	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	wsStore := &mockWorkspaceStore{
		getByID:            func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
	}
	cache := newMockCache()
	chainResolver := resolution.NewChainResolver(wsStore, cache)

	domainStore := &mockDomainStore{
		listInChain: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.Domain, error) {
			return nil, storeErr
		},
	}

	dr := resolution.NewDomainResolver(domainStore, chainResolver, cache)
	err := dr.ValidateFromAddress(context.Background(), wsID, "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, storeErr) {
		t.Errorf("expected store error to propagate, got %v", err)
	}
}

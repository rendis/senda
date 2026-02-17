package resolution_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/resolution"
	"github.com/senda-app/senda/pkg/apperr"
)

// --- Mock WorkspaceStore ---

type mockWorkspaceStore struct {
	getByID            func(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	getSystemWorkspace func(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error)
}

func (m *mockWorkspaceStore) Create(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	return m.getByID(ctx, id)
}
func (m *mockWorkspaceStore) GetByTenantAndCode(_ context.Context, _ uuid.UUID, _ string) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStore) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
	return m.getSystemWorkspace(ctx, tenantID)
}
func (m *mockWorkspaceStore) ListByTenant(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
	return nil, "", nil
}
func (m *mockWorkspaceStore) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStore) SoftDelete(_ context.Context, _ uuid.UUID) error     { return nil }

// --- Mock Cache ---

type mockCache struct {
	data    map[string][]byte
	setErr  error
	setCall int
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string][]byte)}
}

func (m *mockCache) Get(_ context.Context, key string) ([]byte, error) {
	if v, ok := m.data[key]; ok {
		return v, nil
	}
	return nil, apperr.NotFound("cache miss: %s", key)
}

func (m *mockCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.setCall++
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockCache) DeletePattern(_ context.Context, _ string) error {
	return nil
}

// --- Tests ---

func TestChainResolver_RegularWorkspace(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}
	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	store := &mockWorkspaceStore{
		getByID:            func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
	}
	cache := newMockCache()
	resolver := resolution.NewChainResolver(store, cache)

	chain, err := resolver.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chain.WorkspaceID != wsID {
		t.Errorf("WorkspaceID = %v, want %v", chain.WorkspaceID, wsID)
	}
	if chain.SystemWorkspaceID != sysID {
		t.Errorf("SystemWorkspaceID = %v, want %v", chain.SystemWorkspaceID, sysID)
	}
	if chain.TenantID != tenantID {
		t.Errorf("TenantID = %v, want %v", chain.TenantID, tenantID)
	}
	if len(chain.Scopes) != 3 {
		t.Fatalf("Scopes length = %d, want 3", len(chain.Scopes))
	}
	// scope[0] = workspace
	if !chain.Scopes[0].Valid || chain.Scopes[0].UUID != wsID {
		t.Errorf("Scopes[0] = %v, want {UUID: %v, Valid: true}", chain.Scopes[0], wsID)
	}
	// scope[1] = system workspace
	if !chain.Scopes[1].Valid || chain.Scopes[1].UUID != sysID {
		t.Errorf("Scopes[1] = %v, want {UUID: %v, Valid: true}", chain.Scopes[1], sysID)
	}
	// scope[2] = global (NULL)
	if chain.Scopes[2].Valid {
		t.Errorf("Scopes[2].Valid = true, want false (global scope)")
	}
}

func TestChainResolver_SystemWorkspace(t *testing.T) {
	tenantID := uuid.New()
	sysID := uuid.New()

	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	store := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			t.Error("GetSystemWorkspace should not be called for system workspace")
			return nil, nil
		},
	}
	cache := newMockCache()
	resolver := resolution.NewChainResolver(store, cache)

	chain, err := resolver.Resolve(context.Background(), sysID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chain.Scopes) != 2 {
		t.Fatalf("Scopes length = %d, want 2", len(chain.Scopes))
	}
	if !chain.Scopes[0].Valid || chain.Scopes[0].UUID != sysID {
		t.Errorf("Scopes[0] = %v, want {UUID: %v, Valid: true}", chain.Scopes[0], sysID)
	}
	if chain.Scopes[1].Valid {
		t.Errorf("Scopes[1].Valid = true, want false (global scope)")
	}
	// SystemWorkspaceID should equal the workspace itself
	if chain.SystemWorkspaceID != sysID {
		t.Errorf("SystemWorkspaceID = %v, want %v", chain.SystemWorkspaceID, sysID)
	}
}

func TestChainResolver_CacheHit(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	expected := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}

	data, _ := json.Marshal(expected)
	cache := newMockCache()
	cache.data["chain:"+wsID.String()] = data

	storeCalled := false
	store := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			storeCalled = true
			return nil, errors.New("should not be called")
		},
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			storeCalled = true
			return nil, errors.New("should not be called")
		},
	}

	resolver := resolution.NewChainResolver(store, cache)
	chain, err := resolver.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storeCalled {
		t.Error("store was called despite cache hit")
	}
	if chain.WorkspaceID != wsID {
		t.Errorf("WorkspaceID = %v, want %v", chain.WorkspaceID, wsID)
	}
	if len(chain.Scopes) != 3 {
		t.Errorf("Scopes length = %d, want 3", len(chain.Scopes))
	}
}

func TestChainResolver_WorkspaceNotFound(t *testing.T) {
	store := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, apperr.NotFound("workspace not found")
		},
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, nil
		},
	}
	cache := newMockCache()
	resolver := resolution.NewChainResolver(store, cache)

	_, err := resolver.Resolve(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected NotFound AppError, got %v", err)
	}
}

func TestChainResolver_SystemWorkspaceNotFound(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}

	store := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, apperr.NotFound("system workspace not found")
		},
	}
	cache := newMockCache()
	resolver := resolution.NewChainResolver(store, cache)

	_, err := resolver.Resolve(context.Background(), wsID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected NotFound AppError, got %v", err)
	}
}

func TestChainResolver_CacheSetFailureStillReturns(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}
	sysWS := &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}

	store := &mockWorkspaceStore{
		getByID:            func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return sysWS, nil },
	}
	cache := newMockCache()
	cache.setErr = errors.New("cache write failed")

	resolver := resolution.NewChainResolver(store, cache)
	chain, err := resolver.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v (cache set failure should not cause error)", err)
	}
	if chain.WorkspaceID != wsID {
		t.Errorf("WorkspaceID = %v, want %v", chain.WorkspaceID, wsID)
	}
	if len(chain.Scopes) != 3 {
		t.Errorf("Scopes length = %d, want 3", len(chain.Scopes))
	}
}

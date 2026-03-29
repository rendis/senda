package resolution_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
)

// --- Mock workspace store with ListByTenant support ---

type mockWorkspaceStoreWithList struct {
	listByTenant func(ctx context.Context, tenantID uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error)
}

func (m *mockWorkspaceStoreWithList) Create(_ context.Context, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStoreWithList) GetByID(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreWithList) GetByTenantAndCode(_ context.Context, _ uuid.UUID, _ string) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreWithList) GetSystemWorkspace(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreWithList) ListByTenant(ctx context.Context, tenantID uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error) {
	return m.listByTenant(ctx, tenantID, opts)
}
func (m *mockWorkspaceStoreWithList) Update(_ context.Context, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStoreWithList) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }

// --- Mock cache with DeletePattern support (enhances the one in chain_test) ---

type mockCacheWithPattern struct {
	data       map[string][]byte
	deletedKeys []string
}

func newMockCacheWithPattern() *mockCacheWithPattern {
	return &mockCacheWithPattern{data: make(map[string][]byte)}
}

func (m *mockCacheWithPattern) Get(_ context.Context, key string) ([]byte, error) {
	if v, ok := m.data[key]; ok {
		return v, nil
	}
	return nil, errors.New("cache miss")
}

func (m *mockCacheWithPattern) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockCacheWithPattern) Delete(_ context.Context, key string) error {
	m.deletedKeys = append(m.deletedKeys, key)
	delete(m.data, key)
	return nil
}

func (m *mockCacheWithPattern) DeletePattern(_ context.Context, pattern string) error {
	prefix := strings.TrimSuffix(pattern, "*")
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			m.deletedKeys = append(m.deletedKeys, k)
			delete(m.data, k)
		}
	}
	return nil
}

// --- CacheInvalidator Tests ---

func TestCacheInvalidator_InvalidateWorkspace(t *testing.T) {
	wsID := uuid.New()
	cache := newMockCacheWithPattern()
	cacheKey := fmt.Sprintf("chain:%s", wsID.String())
	_ = cache.Set(context.Background(), cacheKey, []byte("data"), time.Minute)

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", nil
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	inv.InvalidateWorkspace(context.Background(), wsID)

	if _, err := cache.Get(context.Background(), cacheKey); err == nil {
		t.Error("expected cache key to be deleted")
	}
}

func TestCacheInvalidator_InvalidateAdapter(t *testing.T) {
	adapterID := uuid.New()
	cache := newMockCacheWithPattern()
	cacheKey := fmt.Sprintf("adapter:%s", adapterID.String())
	_ = cache.Set(context.Background(), cacheKey, []byte("data"), time.Minute)

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", nil
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	inv.InvalidateAdapter(context.Background(), adapterID)

	if _, err := cache.Get(context.Background(), cacheKey); err == nil {
		t.Error("expected cache key to be deleted")
	}
}

func TestCacheInvalidator_InvalidateDomainValidation(t *testing.T) {
	wsID := uuid.New()
	domainName := "example.com"
	cache := newMockCacheWithPattern()
	cacheKey := fmt.Sprintf("domain_valid:%s:%s", wsID.String(), domainName)
	_ = cache.Set(context.Background(), cacheKey, []byte("1"), time.Minute)

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", nil
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	inv.InvalidateDomainValidation(context.Background(), wsID, domainName)

	if _, err := cache.Get(context.Background(), cacheKey); err == nil {
		t.Error("expected cache key to be deleted")
	}
}

func TestCacheInvalidator_InvalidateGlobal(t *testing.T) {
	cache := newMockCacheWithPattern()

	// Populate various cache keys
	ws1 := uuid.New()
	ws2 := uuid.New()
	adapter1 := uuid.New()
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", ws1.String()), []byte("data"), time.Minute)
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", ws2.String()), []byte("data"), time.Minute)
	_ = cache.Set(context.Background(), fmt.Sprintf("adapter:%s", adapter1.String()), []byte("data"), time.Minute)
	// This key should NOT be deleted
	_ = cache.Set(context.Background(), "domain_valid:some:key", []byte("1"), time.Minute)

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", nil
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	inv.InvalidateGlobal(context.Background())

	// chain:* and adapter:* should be gone
	if _, err := cache.Get(context.Background(), fmt.Sprintf("chain:%s", ws1.String())); err == nil {
		t.Error("expected chain key for ws1 to be deleted")
	}
	if _, err := cache.Get(context.Background(), fmt.Sprintf("chain:%s", ws2.String())); err == nil {
		t.Error("expected chain key for ws2 to be deleted")
	}
	if _, err := cache.Get(context.Background(), fmt.Sprintf("adapter:%s", adapter1.String())); err == nil {
		t.Error("expected adapter key to be deleted")
	}
	// domain_valid should remain
	if _, err := cache.Get(context.Background(), "domain_valid:some:key"); err != nil {
		t.Error("expected domain_valid key to remain")
	}
}

func TestCacheInvalidator_InvalidateTenantWorkspaces(t *testing.T) {
	tenantID := uuid.New()
	ws1 := uuid.New()
	ws2 := uuid.New()

	cache := newMockCacheWithPattern()
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", ws1.String()), []byte("data"), time.Minute)
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", ws2.String()), []byte("data"), time.Minute)

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, tid uuid.UUID, opts port.ListOptions) ([]*domain.Workspace, string, error) {
			if tid != tenantID {
				t.Errorf("ListByTenant called with %v, want %v", tid, tenantID)
			}
			if opts.Limit != 1000 {
				t.Errorf("ListByTenant limit = %d, want 1000", opts.Limit)
			}
			return []*domain.Workspace{
				{ID: ws1, TenantID: tenantID},
				{ID: ws2, TenantID: tenantID},
			}, "", nil
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	inv.InvalidateTenantWorkspaces(context.Background(), tenantID)

	if _, err := cache.Get(context.Background(), fmt.Sprintf("chain:%s", ws1.String())); err == nil {
		t.Error("expected chain key for ws1 to be deleted")
	}
	if _, err := cache.Get(context.Background(), fmt.Sprintf("chain:%s", ws2.String())); err == nil {
		t.Error("expected chain key for ws2 to be deleted")
	}
}

func TestCacheInvalidator_InvalidateTenantWorkspaces_ListError(t *testing.T) {
	tenantID := uuid.New()

	cache := newMockCacheWithPattern()

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", errors.New("db error")
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	// Should not panic — best-effort
	inv.InvalidateTenantWorkspaces(context.Background(), tenantID)
}

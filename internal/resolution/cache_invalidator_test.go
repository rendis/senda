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
	listByTenant func(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error)
}

func (m *mockWorkspaceStoreWithList) Create(_ context.Context, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStoreWithList) CreateLogicalPair(_ context.Context, _ *domain.Workspace, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStoreWithList) GetByID(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreWithList) GetByTenantAndCode(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreWithList) GetSystemWorkspace(_ context.Context, _ uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreWithList) ListByTenant(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error) {
	return m.listByTenant(ctx, tenantID, environment, opts)
}
func (m *mockWorkspaceStoreWithList) UpdateShared(_ context.Context, _ uuid.UUID, _, _, _ string) error {
	return nil
}
func (m *mockWorkspaceStoreWithList) Update(_ context.Context, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStoreWithList) SoftDeleteLogical(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockWorkspaceStoreWithList) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockWorkspaceStoreWithList) GetUnsubscribeSigningKey(_ context.Context, _ uuid.UUID) ([]byte, error) {
	return make([]byte, 32), nil
}

// --- Mock cache with DeletePattern support (enhances the one in chain_test) ---

type mockCacheWithPattern struct {
	data                             map[string][]byte
	deletedKeys                      []string
	deletePatternCalls               []string
	deleteResolvedTemplateWorkspaces []uuid.UUID
	deleteAllResolvedTemplatesCalls  int
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
	m.deletePatternCalls = append(m.deletePatternCalls, pattern)
	prefix := strings.TrimSuffix(pattern, "*")
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			m.deletedKeys = append(m.deletedKeys, k)
			delete(m.data, k)
		}
	}
	return nil
}

func (m *mockCacheWithPattern) DeleteResolvedTemplatesByWorkspace(_ context.Context, workspaceID uuid.UUID) error {
	m.deleteResolvedTemplateWorkspaces = append(m.deleteResolvedTemplateWorkspaces, workspaceID)
	prefix := fmt.Sprintf("resolved_template:%s:", workspaceID)
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			m.deletedKeys = append(m.deletedKeys, k)
			delete(m.data, k)
		}
	}
	return nil
}

func (m *mockCacheWithPattern) DeleteAllResolvedTemplates(_ context.Context) error {
	m.deleteAllResolvedTemplatesCalls++
	for k := range m.data {
		if strings.HasPrefix(k, "resolved_template:") {
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
		listByTenant: func(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
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
		listByTenant: func(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
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
		listByTenant: func(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
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
	_ = cache.Set(context.Background(), fmt.Sprintf("resolved_template:%s:welcome:_default", ws1.String()), []byte("data"), time.Minute)
	// This key should NOT be deleted
	_ = cache.Set(context.Background(), "domain_valid:some:key", []byte("1"), time.Minute)

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
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
	if _, err := cache.Get(context.Background(), fmt.Sprintf("resolved_template:%s:welcome:_default", ws1.String())); err == nil {
		t.Error("expected resolved template key to be deleted")
	}
	// domain_valid should remain
	if _, err := cache.Get(context.Background(), "domain_valid:some:key"); err != nil {
		t.Error("expected domain_valid key to remain")
	}
	if cache.deleteAllResolvedTemplatesCalls != 1 {
		t.Fatalf("expected explicit resolved template invalidation once, got %d", cache.deleteAllResolvedTemplatesCalls)
	}
	for _, pattern := range cache.deletePatternCalls {
		if pattern == "resolved_template:*" {
			t.Fatalf("expected global invalidation to avoid resolved_template prefix delete, got %v", cache.deletePatternCalls)
		}
	}
}

func TestCacheInvalidator_InvalidateTenantWorkspaces(t *testing.T) {
	tenantID := uuid.New()
	prodWS := uuid.New()
	testWS := uuid.New()

	cache := newMockCacheWithPattern()
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", prodWS.String()), []byte("data"), time.Minute)
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", testWS.String()), []byte("data"), time.Minute)
	seenEnvironments := make([]domain.Environment, 0, 2)

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, tid uuid.UUID, environment domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error) {
			if tid != tenantID {
				t.Errorf("ListByTenant called with %v, want %v", tid, tenantID)
			}
			if opts.Limit != 1000 {
				t.Errorf("ListByTenant limit = %d, want 1000", opts.Limit)
			}
			seenEnvironments = append(seenEnvironments, environment)
			switch environment {
			case domain.EnvironmentProd:
				return []*domain.Workspace{{ID: prodWS, TenantID: tenantID, Environment: domain.EnvironmentProd}}, "", nil
			case domain.EnvironmentTest:
				return []*domain.Workspace{{ID: testWS, TenantID: tenantID, Environment: domain.EnvironmentTest}}, "", nil
			default:
				return nil, "", fmt.Errorf("unexpected environment %s", environment)
			}
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	inv.InvalidateTenantWorkspaces(context.Background(), tenantID)

	if len(seenEnvironments) != 2 {
		t.Fatalf("expected invalidator to scan 2 environments, got %d (%v)", len(seenEnvironments), seenEnvironments)
	}
	if _, err := cache.Get(context.Background(), fmt.Sprintf("chain:%s", prodWS.String())); err == nil {
		t.Error("expected chain key for prod workspace to be deleted")
	}
	if _, err := cache.Get(context.Background(), fmt.Sprintf("chain:%s", testWS.String())); err == nil {
		t.Error("expected chain key for test workspace to be deleted")
	}
}

func TestCacheInvalidator_InvalidateTenantWorkspaces_PaginatesAllTenantWorkspaces(t *testing.T) {
	tenantID := uuid.New()
	prodWS1 := uuid.New()
	prodWS2 := uuid.New()
	testWS := uuid.New()

	cache := newMockCacheWithPattern()
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", prodWS1.String()), []byte("data"), time.Minute)
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", prodWS2.String()), []byte("data"), time.Minute)
	_ = cache.Set(context.Background(), fmt.Sprintf("chain:%s", testWS.String()), []byte("data"), time.Minute)

	prodCalls := 0
	testCalls := 0
	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, tid uuid.UUID, environment domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error) {
			if tid != tenantID {
				t.Errorf("ListByTenant called with %v, want %v", tid, tenantID)
			}
			switch environment {
			case domain.EnvironmentProd:
				prodCalls++
				switch prodCalls {
				case 1:
					if opts.Cursor != "" {
						t.Fatalf("first prod page cursor = %q, want empty", opts.Cursor)
					}
					return []*domain.Workspace{{ID: prodWS1, TenantID: tenantID, Environment: domain.EnvironmentProd}}, "cursor-prod-2", nil
				case 2:
					if opts.Cursor != "cursor-prod-2" {
						t.Fatalf("second prod page cursor = %q, want %q", opts.Cursor, "cursor-prod-2")
					}
					return []*domain.Workspace{{ID: prodWS2, TenantID: tenantID, Environment: domain.EnvironmentProd}}, "", nil
				default:
					t.Fatalf("unexpected extra prod pagination call %d", prodCalls)
				}
			case domain.EnvironmentTest:
				testCalls++
				if opts.Cursor != "" {
					t.Fatalf("test environment cursor = %q, want empty", opts.Cursor)
				}
				return []*domain.Workspace{{ID: testWS, TenantID: tenantID, Environment: domain.EnvironmentTest}}, "", nil
			default:
				return nil, "", fmt.Errorf("unexpected environment %s", environment)
			}
			return nil, "", nil
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	inv.InvalidateTenantWorkspaces(context.Background(), tenantID)

	for _, wsID := range []uuid.UUID{prodWS1, prodWS2, testWS} {
		if _, err := cache.Get(context.Background(), fmt.Sprintf("chain:%s", wsID.String())); err == nil {
			t.Fatalf("expected cache key for workspace %s to be deleted", wsID)
		}
	}
	if prodCalls != 2 {
		t.Fatalf("expected 2 prod pagination calls, got %d", prodCalls)
	}
	if testCalls != 1 {
		t.Fatalf("expected 1 test pagination call, got %d", testCalls)
	}
}

func TestCacheInvalidator_InvalidateTenantWorkspaces_ListError(t *testing.T) {
	tenantID := uuid.New()

	cache := newMockCacheWithPattern()

	wsStore := &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", errors.New("db error")
		},
	}

	inv := resolution.NewCacheInvalidator(cache, wsStore)
	// Should not panic — best-effort
	inv.InvalidateTenantWorkspaces(context.Background(), tenantID)
}

func TestCacheInvalidator_InvalidateResolvedTemplates_UsesExplicitWorkspaceScope(t *testing.T) {
	wsID := uuid.New()
	cache := newMockCacheWithPattern()
	targetKey := fmt.Sprintf("resolved_template:%s:welcome:_default", wsID)
	otherKey := fmt.Sprintf("resolved_template:%s:receipt:es", uuid.New())
	_ = cache.Set(context.Background(), targetKey, []byte("welcome"), time.Minute)
	_ = cache.Set(context.Background(), otherKey, []byte("receipt"), time.Minute)

	inv := resolution.NewCacheInvalidator(cache, &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", nil
		},
	})
	inv.InvalidateResolvedTemplates(context.Background(), wsID)

	if len(cache.deleteResolvedTemplateWorkspaces) != 1 || cache.deleteResolvedTemplateWorkspaces[0] != wsID {
		t.Fatalf("expected explicit workspace invalidation for %s, got %v", wsID, cache.deleteResolvedTemplateWorkspaces)
	}
	if len(cache.deletePatternCalls) != 0 {
		t.Fatalf("expected no DeletePattern fallback, got %v", cache.deletePatternCalls)
	}
	if _, err := cache.Get(context.Background(), targetKey); err == nil {
		t.Fatalf("expected resolved template key %q to be deleted", targetKey)
	}
	if _, err := cache.Get(context.Background(), otherKey); err != nil {
		t.Fatalf("expected other workspace key %q to remain, got %v", otherKey, err)
	}
}

func TestCacheInvalidator_InvalidateAllResolvedTemplates_UsesExplicitScopeDeletion(t *testing.T) {
	cache := newMockCacheWithPattern()
	_ = cache.Set(context.Background(), fmt.Sprintf("resolved_template:%s:welcome:_default", uuid.New()), []byte("welcome"), time.Minute)
	_ = cache.Set(context.Background(), fmt.Sprintf("resolved_template:%s:receipt:es", uuid.New()), []byte("receipt"), time.Minute)
	_ = cache.Set(context.Background(), "chain:keep-me", []byte("chain"), time.Minute)

	inv := resolution.NewCacheInvalidator(cache, &mockWorkspaceStoreWithList{
		listByTenant: func(_ context.Context, _ uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
			return nil, "", nil
		},
	})
	inv.InvalidateAllResolvedTemplates(context.Background())

	if cache.deleteAllResolvedTemplatesCalls != 1 {
		t.Fatalf("expected DeleteAllResolvedTemplates once, got %d", cache.deleteAllResolvedTemplatesCalls)
	}
	if len(cache.deletePatternCalls) != 0 {
		t.Fatalf("expected no DeletePattern fallback, got %v", cache.deletePatternCalls)
	}
	if _, err := cache.Get(context.Background(), "chain:keep-me"); err != nil {
		t.Fatalf("expected non-template cache key to remain, got %v", err)
	}
}

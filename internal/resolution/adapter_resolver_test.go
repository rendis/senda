package resolution_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
)

// --- Mock AdapterStore ---

type mockAdapterStore struct {
	getByID func(ctx context.Context, id uuid.UUID) (*domain.Adapter, error)
}

func (m *mockAdapterStore) Create(_ context.Context, _ *domain.Adapter) error { return nil }
func (m *mockAdapterStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error) {
	return m.getByID(ctx, id)
}
func (m *mockAdapterStore) Update(_ context.Context, _ *domain.Adapter) error    { return nil }
func (m *mockAdapterStore) SoftDelete(_ context.Context, _ uuid.UUID) error      { return nil }
func (m *mockAdapterStore) ListInChain(_ context.Context, _ []uuid.NullUUID) ([]*domain.Adapter, error) {
	return nil, nil
}
func (m *mockAdapterStore) ListByWorkspace(_ context.Context, _ *uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	return nil, nil
}

// --- Tests ---

func TestAdapterResolver_NilAdapterID(t *testing.T) {
	store := &mockAdapterStore{}
	cache := newMockCache()

	resolver := resolution.NewAdapterResolver(store, cache)
	tt := &domain.TemplateType{ID: uuid.New(), AdapterID: nil}

	_, err := resolver.ResolveForTemplateType(context.Background(), tt)
	if !errors.Is(err, domain.ErrNoAdapterConfigured) {
		t.Errorf("expected ErrNoAdapterConfigured, got %v", err)
	}
}

func TestAdapterResolver_CacheMiss_FetchFromStore(t *testing.T) {
	adapterID := uuid.New()
	adapter := &domain.Adapter{ID: adapterID, Name: "ses-prod"}

	storeCalled := false
	store := &mockAdapterStore{
		getByID: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			storeCalled = true
			if id != adapterID {
				t.Errorf("GetByID called with %v, want %v", id, adapterID)
			}
			return adapter, nil
		},
	}
	cache := newMockCache()

	resolver := resolution.NewAdapterResolver(store, cache)
	tt := &domain.TemplateType{ID: uuid.New(), AdapterID: &adapterID}

	result, err := resolver.ResolveForTemplateType(context.Background(), tt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !storeCalled {
		t.Error("store.GetByID was not called")
	}
	if result.Adapter.ID != adapterID {
		t.Errorf("Adapter.ID = %v, want %v", result.Adapter.ID, adapterID)
	}
	// Verify cache was populated
	if cache.setCall == 0 {
		t.Error("cache.Set was not called")
	}
}

func TestAdapterResolver_CacheHit(t *testing.T) {
	adapterID := uuid.New()
	adapter := &domain.Adapter{ID: adapterID, Name: "ses-prod"}

	storeCalled := false
	store := &mockAdapterStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			storeCalled = true
			return nil, errors.New("should not be called")
		},
	}
	cache := newMockCache()

	// Pre-populate cache
	resolver := resolution.NewAdapterResolver(store, cache)
	resolution.SeedAdapterCache(cache, adapterID, adapter)

	tt := &domain.TemplateType{ID: uuid.New(), AdapterID: &adapterID}

	result, err := resolver.ResolveForTemplateType(context.Background(), tt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storeCalled {
		t.Error("store.GetByID was called despite cache hit")
	}
	if result.Adapter.ID != adapterID {
		t.Errorf("Adapter.ID = %v, want %v", result.Adapter.ID, adapterID)
	}
}

func TestAdapterResolver_SoftDeletedAdapter(t *testing.T) {
	adapterID := uuid.New()
	now := time.Now()
	adapter := &domain.Adapter{ID: adapterID, Name: "ses-prod", DeletedAt: &now}

	store := &mockAdapterStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			return adapter, nil
		},
	}
	cache := newMockCache()

	resolver := resolution.NewAdapterResolver(store, cache)
	tt := &domain.TemplateType{ID: uuid.New(), AdapterID: &adapterID}

	_, err := resolver.ResolveForTemplateType(context.Background(), tt)
	if !errors.Is(err, domain.ErrNoAdapterConfigured) {
		t.Errorf("expected ErrNoAdapterConfigured, got %v", err)
	}
}

func TestAdapterResolver_StoreError(t *testing.T) {
	adapterID := uuid.New()
	storeErr := errors.New("db connection failed")

	store := &mockAdapterStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			return nil, storeErr
		},
	}
	cache := newMockCache()

	resolver := resolution.NewAdapterResolver(store, cache)
	tt := &domain.TemplateType{ID: uuid.New(), AdapterID: &adapterID}

	_, err := resolver.ResolveForTemplateType(context.Background(), tt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, storeErr) {
		t.Errorf("expected store error to propagate, got %v", err)
	}
}

package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

const adapterCacheTTL = 10 * time.Minute

// ResolvedAdapter holds the resolved adapter for a template type.
type ResolvedAdapter struct {
	Adapter *domain.Adapter
}

// AdapterResolver resolves the adapter assigned to a template type.
type AdapterResolver struct {
	adapterStore port.AdapterStore
	cache        port.Cache
}

// NewAdapterResolver creates an AdapterResolver with the given dependencies.
func NewAdapterResolver(store port.AdapterStore, cache port.Cache) *AdapterResolver {
	return &AdapterResolver{
		adapterStore: store,
		cache:        cache,
	}
}

// ResolveForTemplateType returns the adapter assigned to the given template type.
func (r *AdapterResolver) ResolveForTemplateType(ctx context.Context, templateType *domain.TemplateType) (*ResolvedAdapter, error) {
	if templateType.AdapterID == nil {
		return nil, domain.ErrNoAdapterConfigured
	}

	adapterID := *templateType.AdapterID
	cacheKey := adapterCacheKey(adapterID)

	// Try cache
	if data, err := r.cache.Get(ctx, cacheKey); err == nil {
		var adapter domain.Adapter
		if err := json.Unmarshal(data, &adapter); err == nil {
			if adapter.DeletedAt != nil {
				return nil, domain.ErrNoAdapterConfigured
			}
			return &ResolvedAdapter{Adapter: &adapter}, nil
		}
	}

	// Cache miss — fetch from store
	adapter, err := r.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}

	if adapter.DeletedAt != nil {
		return nil, domain.ErrNoAdapterConfigured
	}

	// Best-effort cache write
	if data, err := json.Marshal(adapter); err == nil {
		_ = r.cache.Set(ctx, cacheKey, data, adapterCacheTTL)
	}

	return &ResolvedAdapter{Adapter: adapter}, nil
}

func adapterCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("adapter:%s", id.String())
}

// SeedAdapterCache is a test helper that pre-populates the adapter cache.
func SeedAdapterCache(cache port.Cache, id uuid.UUID, adapter *domain.Adapter) {
	data, err := json.Marshal(adapter)
	if err != nil {
		return
	}
	_ = cache.Set(context.Background(), adapterCacheKey(id), data, adapterCacheTTL)
}

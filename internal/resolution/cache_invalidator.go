package resolution

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/port"
)

// CacheInvalidator provides methods to invalidate resolution cache entries
// when underlying data changes.
type CacheInvalidator struct {
	cache          port.Cache
	workspaceStore port.WorkspaceStore
}

// NewCacheInvalidator creates a CacheInvalidator with the given dependencies.
func NewCacheInvalidator(cache port.Cache, ws port.WorkspaceStore) *CacheInvalidator {
	return &CacheInvalidator{
		cache:          cache,
		workspaceStore: ws,
	}
}

// InvalidateWorkspace removes the cached resolution chain for a workspace.
func (c *CacheInvalidator) InvalidateWorkspace(ctx context.Context, workspaceID uuid.UUID) {
	_ = c.cache.Delete(ctx, fmt.Sprintf("chain:%s", workspaceID.String()))
}

// InvalidateAdapter removes the cached adapter data.
func (c *CacheInvalidator) InvalidateAdapter(ctx context.Context, adapterID uuid.UUID) {
	_ = c.cache.Delete(ctx, fmt.Sprintf("adapter:%s", adapterID.String()))
}

// InvalidateDomainValidation removes the cached domain validation result
// for a specific workspace and domain name.
func (c *CacheInvalidator) InvalidateDomainValidation(ctx context.Context, workspaceID uuid.UUID, domainName string) {
	_ = c.cache.Delete(ctx, fmt.Sprintf("domain_valid:%s:%s", workspaceID.String(), domainName))
}

// InvalidateTenantWorkspaces removes cached resolution chains for all
// workspaces belonging to a tenant. This is best-effort: list errors are ignored.
func (c *CacheInvalidator) InvalidateTenantWorkspaces(ctx context.Context, tenantID uuid.UUID) {
	workspaces, _, err := c.workspaceStore.ListByTenant(ctx, tenantID, port.ListOptions{Limit: 1000})
	if err != nil {
		return
	}
	for _, ws := range workspaces {
		c.InvalidateWorkspace(ctx, ws.ID)
	}
}

// InvalidateGlobal removes all cached resolution chains and adapter data.
func (c *CacheInvalidator) InvalidateGlobal(ctx context.Context) {
	_ = c.cache.DeletePattern(ctx, "chain:*")
	_ = c.cache.DeletePattern(ctx, "adapter:*")
}

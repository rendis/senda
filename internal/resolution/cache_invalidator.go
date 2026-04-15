package resolution

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// CacheInvalidator provides methods to invalidate resolution cache entries
// when underlying data changes.
type CacheInvalidator struct {
	cache          port.Cache
	workspaceStore port.WorkspaceStore
}

type resolvedTemplateScopeCache interface {
	DeleteResolvedTemplatesByWorkspace(ctx context.Context, workspaceID uuid.UUID) error
	DeleteAllResolvedTemplates(ctx context.Context) error
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
// workspaces belonging to a tenant. If listing workspaces fails, it falls back
// to a global invalidation so stale data is never served.
func (c *CacheInvalidator) InvalidateTenantWorkspaces(ctx context.Context, tenantID uuid.UUID) {
	for _, environment := range domain.Environments() {
		cursor := ""
		for {
			workspaces, nextCursor, err := c.workspaceStore.ListByTenant(ctx, tenantID, environment, port.ListOptions{
				Limit:  1000,
				Cursor: cursor,
			})
			if err != nil {
				slog.Error("failed to list workspaces for cache invalidation, falling back to global",
					"tenant_id", tenantID, "environment", environment, "error", err)
				c.InvalidateGlobal(ctx)
				return
			}
			for _, ws := range workspaces {
				c.InvalidateWorkspace(ctx, ws.ID)
			}
			if nextCursor == "" {
				break
			}
			cursor = nextCursor
		}
	}
}

// InvalidateResolvedTemplates removes cached resolved templates for a workspace.
// Call this when a template is published, disabled, or its type changes.
func (c *CacheInvalidator) InvalidateResolvedTemplates(ctx context.Context, workspaceID uuid.UUID) {
	cache, ok := c.cache.(resolvedTemplateScopeCache)
	if !ok {
		slog.Warn("resolved template cache does not support explicit workspace invalidation", "workspace_id", workspaceID)
		return
	}
	_ = cache.DeleteResolvedTemplatesByWorkspace(ctx, workspaceID)
}

// InvalidateAllResolvedTemplates removes all cached resolved templates across all workspaces.
func (c *CacheInvalidator) InvalidateAllResolvedTemplates(ctx context.Context) {
	cache, ok := c.cache.(resolvedTemplateScopeCache)
	if !ok {
		slog.Warn("resolved template cache does not support explicit global invalidation")
		return
	}
	_ = cache.DeleteAllResolvedTemplates(ctx)
}

// InvalidateGlobal removes all cached resolution chains, adapter data, and resolved templates.
func (c *CacheInvalidator) InvalidateGlobal(ctx context.Context) {
	_ = c.cache.DeletePattern(ctx, "chain:*")
	_ = c.cache.DeletePattern(ctx, "adapter:*")
	c.InvalidateAllResolvedTemplates(ctx)
}

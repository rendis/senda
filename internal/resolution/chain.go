package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/port"
)

const chainCacheTTL = 5 * time.Minute

// ResolutionChain holds the ordered scopes for tenant-local hierarchical resolution.
// Scopes are ordered from highest priority (workspace) to lowest (_system).
type ResolutionChain struct {
	WorkspaceID       uuid.UUID       `json:"workspace_id"`
	SystemWorkspaceID uuid.UUID       `json:"system_workspace_id"`
	TenantID          uuid.UUID       `json:"tenant_id"`
	Scopes            []uuid.NullUUID `json:"scopes"`
}

// ChainResolver builds a ResolutionChain for a given workspace,
// using the tenant-local hierarchy: workspace → _system.
type ChainResolver struct {
	workspaceStore port.WorkspaceStore
	cache          port.Cache
}

// NewChainResolver creates a ChainResolver with the given dependencies.
func NewChainResolver(ws port.WorkspaceStore, cache port.Cache) *ChainResolver {
	return &ChainResolver{
		workspaceStore: ws,
		cache:          cache,
	}
}

// Resolve returns the resolution chain for the given workspace ID.
// It checks the cache first, falling back to DB lookups on miss.
func (r *ChainResolver) Resolve(ctx context.Context, workspaceID uuid.UUID) (*ResolutionChain, error) {
	cacheKey := fmt.Sprintf("chain:%s", workspaceID.String())

	// Try cache
	if data, err := r.cache.Get(ctx, cacheKey); err == nil {
		var chain ResolutionChain
		if err := json.Unmarshal(data, &chain); err == nil {
			return &chain, nil
		}
	}

	// Cache miss — load from store
	ws, err := r.workspaceStore.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var chain *ResolutionChain

	if ws.IsSystem {
		chain = &ResolutionChain{
			WorkspaceID:       ws.ID,
			SystemWorkspaceID: ws.ID,
			TenantID:          ws.TenantID,
			Scopes: []uuid.NullUUID{
				{UUID: ws.ID, Valid: true},
			},
		}
	} else {
		sysWS, err := r.workspaceStore.GetSystemWorkspace(ctx, ws.TenantID)
		if err != nil {
			return nil, err
		}
		chain = &ResolutionChain{
			WorkspaceID:       ws.ID,
			SystemWorkspaceID: sysWS.ID,
			TenantID:          ws.TenantID,
			Scopes: []uuid.NullUUID{
				{UUID: ws.ID, Valid: true},
				{UUID: sysWS.ID, Valid: true},
			},
		}
	}

	// Best-effort cache write
	if data, err := json.Marshal(chain); err == nil {
		_ = r.cache.Set(ctx, cacheKey, data, chainCacheTTL)
	}

	return chain, nil
}

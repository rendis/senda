package middleware

import (
	"context"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// workspaceFilter is a per-request implementation of port.WorkspaceFilter that
// binds a tenant code and environment to the underlying existence store.
// It is constructed fresh for each request to prevent tenant/environment leakage
// between concurrent requests.
type workspaceFilter struct {
	store       port.WorkspaceExistenceStore
	tenantCode  string
	environment domain.Environment
}

func newWorkspaceFilter(store port.WorkspaceExistenceStore, tenantCode string, environment domain.Environment) *workspaceFilter {
	return &workspaceFilter{
		store:       store,
		tenantCode:  tenantCode,
		environment: environment,
	}
}

// Exists returns a dense map where each requested code is a key. Codes that
// are active for the tenant are mapped to true; unknown or deleted codes map to
// false. An empty/nil codes slice returns an empty map without contacting the
// store (REQ-B4).
func (f *workspaceFilter) Exists(ctx context.Context, codes []string) (map[string]bool, error) {
	if len(codes) == 0 {
		return map[string]bool{}, nil
	}
	return f.store.ExistsActiveByTenantCode(ctx, f.tenantCode, codes, f.environment)
}

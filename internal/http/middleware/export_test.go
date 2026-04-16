package middleware

import (
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// NewWorkspaceFilterForTest exposes newWorkspaceFilter for external test packages.
func NewWorkspaceFilterForTest(store port.WorkspaceExistenceStore, tenantCode string, environment domain.Environment) *workspaceFilter {
	return newWorkspaceFilter(store, tenantCode, environment)
}

package postgres

import "github.com/senda-app/senda/internal/port"

// Compile-time interface satisfaction checks.
var (
	_ port.TenantStore       = (*TenantRepo)(nil)
	_ port.WorkspaceStore    = (*WorkspaceRepo)(nil)
	_ port.GlobalConfigStore = (*GlobalConfigRepo)(nil)
)

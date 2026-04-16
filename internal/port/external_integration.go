package port

import (
	"context"

	"github.com/rendis/senda/internal/domain"
)

// ExternalIntegrationRequest carries the request context for external
// integration auth and workspace resolution flows.
type ExternalIntegrationRequest struct {
	ProfileSlug    string
	Environment    domain.Environment
	TenantCode     string
	WorkspaceCodes []string
	Token          string
	Headers        map[string]string
	QueryParams    map[string]string
	Path           string
	Method         string
}

// ExternalPermissions captures the capabilities granted by the auth method.
// The external surface enforces these booleans together with the profile
// capabilities configured in system settings.
type ExternalPermissions struct {
	ListTemplates   bool
	ViewVersions    bool
	EditVersions    bool
	PublishVersions bool
	TestSend        bool
	BuilderAccess   bool
	MetadataAccess  bool
	LocaleAccess    bool
}

// ExternalAuthResult is the normalized auth/access context returned by a
// custom external auth method.
type ExternalAuthResult struct {
	Permissions ExternalPermissions
	Context     map[string]any
}

// ExternalWorkspaceResolution is the outcome of a custom workspace resolver.
// When ReadOnly is true, the caller must use the tenant _system workspace
// and disallow all mutating operations.
type ExternalWorkspaceResolution struct {
	WorkspaceCode string
	ReadOnly      bool
}

// ExternalAuthMethod validates the incoming external integration request and
// returns a normalized context and permissions set.
type ExternalAuthMethod interface {
	Name() string
	Description() string
	Authenticate(ctx context.Context, req *ExternalIntegrationRequest) (*ExternalAuthResult, error)
}

// WorkspaceFilter provides a read-only view of which workspace codes are active
// for the current tenant and environment. It is constructed per-request by the
// middleware and injected into ExternalWorkspaceResolver.
type WorkspaceFilter interface {
	// Exists returns a dense map where every requested code is present as a key.
	// A code maps to true only when the workspace is active for the tenant in the
	// current environment. An empty or nil codes slice returns an empty map
	// without contacting the store.
	Exists(ctx context.Context, codes []string) (map[string]bool, error)
}

// ExternalWorkspaceResolver maps the authenticated request to a workspace
// code or a read-only fallback. filter provides an existence check for
// workspace codes scoped to the current tenant and environment.
type ExternalWorkspaceResolver interface {
	Name() string
	Description() string
	ResolveWorkspace(ctx context.Context, req *ExternalIntegrationRequest, auth *ExternalAuthResult, filter WorkspaceFilter) (*ExternalWorkspaceResolution, error)
}

package sdk

import (
	"context"
	"time"

	"github.com/rendis/senda/internal/domain"
)

// InjectorFieldType is the public field type enum for injector catalog fields.
type InjectorFieldType string

const (
	FieldTypeText   InjectorFieldType = InjectorFieldType(domain.FieldTypeText)
	FieldTypeNumber InjectorFieldType = InjectorFieldType(domain.FieldTypeNumber)
	FieldTypeBool   InjectorFieldType = InjectorFieldType(domain.FieldTypeBool)
	FieldTypeImg    InjectorFieldType = InjectorFieldType(domain.FieldTypeImg)
	FieldTypeURL    InjectorFieldType = InjectorFieldType(domain.FieldTypeURL)
	FieldTypeHTML   InjectorFieldType = InjectorFieldType(domain.FieldTypeHTML)
)

// ResolveFunc executes injector logic and returns field-name → value pairs.
type ResolveFunc func(ctx context.Context, injCtx *InjectorContext) (map[string]any, error)

// InjectorFieldSpec describes one field exposed by a catalogable injector.
type InjectorFieldSpec struct {
	Name        string
	Type        InjectorFieldType
	Description string
}

// InjectorRegistration is the single public registration contract for
// injectors. Static registrations are catalogable and read-only in the UI.
type InjectorRegistration struct {
	Code         string
	Name         string
	Description  string
	Static       bool
	TTL          time.Duration
	Fields       []InjectorFieldSpec
	Resolve      ResolveFunc
	Dependencies []string
	Critical     bool
	Timeout      time.Duration
}

// InitFunc runs once per send request before code injectors.
type InitFunc func(ctx context.Context, injCtx *InjectorContext) (any, error)

// ExternalAuthMethod validates external integration requests and returns a
// normalized access context.
type ExternalAuthMethod interface {
	Name() string
	Description() string
	Authenticate(ctx context.Context, req *ExternalIntegrationRequest) (*ExternalAuthResult, error)
}

// WorkspaceFilter provides callers with a read-only view of which workspace
// codes are active for the current tenant and environment. It is constructed
// per-request by the middleware and injected into ExternalWorkspaceResolver.
type WorkspaceFilter interface {
	// Exists returns a dense map where every requested code is present as a key.
	// A code maps to true only when the workspace is active for the tenant in the
	// current environment. An empty or nil codes slice returns an empty map
	// without contacting the store.
	Exists(ctx context.Context, codes []string) (map[string]bool, error)
}

// ExternalWorkspaceResolver maps a validated request to a workspace code or a
// read-only fallback. filter provides an existence check for workspace codes
// scoped to the current tenant and environment.
type ExternalWorkspaceResolver interface {
	Name() string
	Description() string
	ResolveWorkspace(ctx context.Context, req *ExternalIntegrationRequest, auth *ExternalAuthResult, filter WorkspaceFilter) (*ExternalWorkspaceResolution, error)
}

// ExternalIntegrationRequest is the request context passed to auth and
// resolver extensions.
type ExternalIntegrationRequest struct {
	ProfileSlug    string
	Environment    Environment
	TenantCode     string
	WorkspaceCodes []string
	Token          string
	Headers        map[string]string
	QueryParams    map[string]string
	Path           string
	Method         string
}

// ExternalPermissions describes the capability flags returned by an auth
// method.
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

// ExternalAuthResult is the normalized access context returned by auth.
type ExternalAuthResult struct {
	Permissions ExternalPermissions
	Context     map[string]any
}

// ExternalWorkspaceResolution is the result of a workspace resolver.
type ExternalWorkspaceResolution struct {
	WorkspaceCode string
	ReadOnly      bool
}

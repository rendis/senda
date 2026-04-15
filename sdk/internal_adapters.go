package sdk

import (
	"context"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

func adaptInitFunc(fn InitFunc) port.CodeInitFunc {
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, injCtx *port.InjectorContext) (any, error) {
		return fn(ctx, wrapInjectorContext(injCtx))
	}
}

func adaptExternalAuthMethods(methods []ExternalAuthMethod) []port.ExternalAuthMethod {
	if len(methods) == 0 {
		return nil
	}
	adapted := make([]port.ExternalAuthMethod, 0, len(methods))
	for _, method := range methods {
		adapted = append(adapted, externalAuthMethodAdapter{method: method})
	}
	return adapted
}

func adaptExternalWorkspaceResolvers(resolvers []ExternalWorkspaceResolver) []port.ExternalWorkspaceResolver {
	if len(resolvers) == 0 {
		return nil
	}
	adapted := make([]port.ExternalWorkspaceResolver, 0, len(resolvers))
	for _, resolver := range resolvers {
		adapted = append(adapted, externalWorkspaceResolverAdapter{resolver: resolver})
	}
	return adapted
}

func adaptInjectorFieldSpecs(fields []InjectorFieldSpec) []port.InjectorFieldSpec {
	if len(fields) == 0 {
		return nil
	}
	adapted := make([]port.InjectorFieldSpec, 0, len(fields))
	for _, field := range fields {
		adapted = append(adapted, port.InjectorFieldSpec{
			Name:        field.Name,
			Type:        domain.InjectorFieldType(field.Type),
			Description: field.Description,
		})
	}
	return adapted
}

func adaptExternalRequest(req *port.ExternalIntegrationRequest) *ExternalIntegrationRequest {
	if req == nil {
		return nil
	}
	workspaceCodes := make([]string, len(req.WorkspaceCodes))
	copy(workspaceCodes, req.WorkspaceCodes)
	headers := make(map[string]string, len(req.Headers))
	for k, v := range req.Headers {
		headers[k] = v
	}
	queryParams := make(map[string]string, len(req.QueryParams))
	for k, v := range req.QueryParams {
		queryParams[k] = v
	}
	return &ExternalIntegrationRequest{
		ProfileSlug:    req.ProfileSlug,
		Environment:    fromDomainEnvironment(req.Environment),
		TenantCode:     req.TenantCode,
		WorkspaceCodes: workspaceCodes,
		Token:          req.Token,
		Headers:        headers,
		QueryParams:    queryParams,
		Path:           req.Path,
		Method:         req.Method,
	}
}

func adaptExternalAuthResult(result *ExternalAuthResult) *port.ExternalAuthResult {
	if result == nil {
		return nil
	}
	contextMap := make(map[string]any, len(result.Context))
	for k, v := range result.Context {
		contextMap[k] = v
	}
	return &port.ExternalAuthResult{
		Permissions: port.ExternalPermissions{
			ListTemplates:   result.Permissions.ListTemplates,
			ViewVersions:    result.Permissions.ViewVersions,
			EditVersions:    result.Permissions.EditVersions,
			PublishVersions: result.Permissions.PublishVersions,
			TestSend:        result.Permissions.TestSend,
			BuilderAccess:   result.Permissions.BuilderAccess,
			MetadataAccess:  result.Permissions.MetadataAccess,
			LocaleAccess:    result.Permissions.LocaleAccess,
		},
		Context: contextMap,
	}
}

func adaptExternalWorkspaceResolution(result *ExternalWorkspaceResolution) *port.ExternalWorkspaceResolution {
	if result == nil {
		return nil
	}
	return &port.ExternalWorkspaceResolution{
		WorkspaceCode: result.WorkspaceCode,
		ReadOnly:      result.ReadOnly,
	}
}

type externalAuthMethodAdapter struct {
	method ExternalAuthMethod
}

func (a externalAuthMethodAdapter) Name() string        { return a.method.Name() }
func (a externalAuthMethodAdapter) Description() string { return a.method.Description() }
func (a externalAuthMethodAdapter) Authenticate(ctx context.Context, req *port.ExternalIntegrationRequest) (*port.ExternalAuthResult, error) {
	result, err := a.method.Authenticate(ctx, adaptExternalRequest(req))
	if err != nil {
		return nil, err
	}
	return adaptExternalAuthResult(result), nil
}

type externalWorkspaceResolverAdapter struct {
	resolver ExternalWorkspaceResolver
}

func (a externalWorkspaceResolverAdapter) Name() string        { return a.resolver.Name() }
func (a externalWorkspaceResolverAdapter) Description() string { return a.resolver.Description() }
func (a externalWorkspaceResolverAdapter) ResolveWorkspace(ctx context.Context, req *port.ExternalIntegrationRequest, auth *port.ExternalAuthResult) (*port.ExternalWorkspaceResolution, error) {
	result, err := a.resolver.ResolveWorkspace(ctx, adaptExternalRequest(req), &ExternalAuthResult{
		Permissions: ExternalPermissions{
			ListTemplates:   auth.Permissions.ListTemplates,
			ViewVersions:    auth.Permissions.ViewVersions,
			EditVersions:    auth.Permissions.EditVersions,
			PublishVersions: auth.Permissions.PublishVersions,
			TestSend:        auth.Permissions.TestSend,
			BuilderAccess:   auth.Permissions.BuilderAccess,
			MetadataAccess:  auth.Permissions.MetadataAccess,
			LocaleAccess:    auth.Permissions.LocaleAccess,
		},
		Context: auth.Context,
	})
	if err != nil {
		return nil, err
	}
	return adaptExternalWorkspaceResolution(result), nil
}

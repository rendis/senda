package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

const (
	// ContextKeyExternalIntegrationProfile stores the loaded external profile.
	ContextKeyExternalIntegrationProfile = "external_integration_profile"

	// ContextKeyExternalIntegrationAuthResult stores the auth result returned by the selected method.
	ContextKeyExternalIntegrationAuthResult = "external_integration_auth_result"

	// ContextKeyExternalIntegrationResolution stores the workspace resolution returned by the selected resolver.
	ContextKeyExternalIntegrationResolution = "external_integration_resolution"

	// ContextKeyExternalIntegrationPermissions stores the effective permissions after combining profile + auth.
	ContextKeyExternalIntegrationPermissions = "external_integration_permissions"

	// ContextKeyExternalIntegrationReadOnly stores whether the request is running in read-only fallback mode.
	ContextKeyExternalIntegrationReadOnly = "external_integration_read_only"

	// ContextKeyExternalIntegrationEffectiveWorkspaceCode stores the workspace code handlers should use.
	ContextKeyExternalIntegrationEffectiveWorkspaceCode = "external_integration_effective_workspace_code"

	// ContextKeyExternalIntegrationAllowedHeaders stores the explicit profile-allowed
	// headers captured from the incoming request for downstream use.
	ContextKeyExternalIntegrationAllowedHeaders = "external_integration_allowed_headers"

	ExternalIntegrationEnvironmentHeader = "X-Senda-Environment"
)

// ExternalAction identifies the capability required by a given external route.
type ExternalAction string

const (
	ExternalActionListTemplates   ExternalAction = "list_templates"
	ExternalActionViewVersions    ExternalAction = "view_versions"
	ExternalActionEditVersions    ExternalAction = "edit_versions"
	ExternalActionPublishVersions ExternalAction = "publish_versions"
	ExternalActionTestSend        ExternalAction = "test_send"
	ExternalActionBuilderAccess   ExternalAction = "builder_access"
	ExternalActionMetadataAccess  ExternalAction = "metadata_access"
	ExternalActionLocaleAccess    ExternalAction = "locale_access"
)

var errExternalAuthDenied = errors.New("external integration auth denied")

const ExternalIntegrationTokenHeader = "x-senda-external-token"

// ExternalIntegrationResolverStore is the minimal contract required by the
// external integration middleware. It is implemented by the handler layer.
type ExternalIntegrationResolverStore interface {
	LoadProfileBySlug(ctx context.Context, slug string) (domain.ExternalIntegrationProfile, error)
	AuthMethodByName(name string) (port.ExternalAuthMethod, bool)
	ResolverByName(name string) (port.ExternalWorkspaceResolver, bool)
}

// ExternalIntegration resolves the external profile, auth method and workspace
// resolver for a request, and stores the resulting state in the echo context.
// store is used to construct a per-request WorkspaceFilter bound to the current
// tenant and environment; it may be nil for deployments that do not require
// workspace existence checks.
func ExternalIntegration(h ExternalIntegrationResolverStore, store port.WorkspaceExistenceStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			profile, err := loadExternalProfileForRequest(c, h)
			if err != nil || responseCommitted(c) {
				return err
			}
			if err := validateExternalEnvironment(c); err != nil || responseCommitted(c) {
				return err
			}
			if err := validateExternalRequiredHeaders(c, profile); err != nil || responseCommitted(c) {
				return err
			}
			if err := validateExternalTokenTransport(c); err != nil || responseCommitted(c) {
				return err
			}

			authMethod, resolver, err := resolveExternalDependencies(c, h, profile)
			if err != nil || responseCommitted(c) {
				return err
			}

			reqCtx := newExternalIntegrationRequest(c, profile)
			authResult, err := authenticateExternalRequest(c, authMethod, reqCtx)
			if err != nil || responseCommitted(c) {
				return err
			}

			resolution, effectiveWorkspaceCode, readOnly, err := resolveExternalWorkspace(c, resolver, reqCtx, authResult, buildWorkspaceFilter(store, reqCtx))
			if err != nil || responseCommitted(c) {
				return err
			}

			applyExternalIntegrationContext(c, profile, reqCtx, authResult, resolution, effectiveWorkspaceCode, readOnly)
			return next(c)
		}
	}
}

func loadExternalProfileForRequest(c *echo.Context, h ExternalIntegrationResolverStore) (domain.ExternalIntegrationProfile, error) {
	profileSlug := strings.TrimSpace(c.Param("profile_slug"))
	if profileSlug == "" {
		return domain.ExternalIntegrationProfile{}, response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "missing profile slug")
	}

	profile, err := h.LoadProfileBySlug(c.Request().Context(), profileSlug)
	if err != nil {
		return domain.ExternalIntegrationProfile{}, mapExternalProfileLoadError(c, err)
	}

	return profile, nil
}

func validateExternalRequiredHeaders(c *echo.Context, profile domain.ExternalIntegrationProfile) error {
	for _, header := range profile.RequiredHeaders {
		if c.Request().Header.Get(header) == "" {
			return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing required header "+header)
		}
	}

	return nil
}

func resolveExternalDependencies(c *echo.Context, h ExternalIntegrationResolverStore, profile domain.ExternalIntegrationProfile) (port.ExternalAuthMethod, port.ExternalWorkspaceResolver, error) {
	authMethod, ok := h.AuthMethodByName(profile.AuthMethodName)
	if !ok {
		return nil, nil, response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "external auth method not registered")
	}

	resolver, ok := h.ResolverByName(profile.ResolverName)
	if !ok {
		return nil, nil, response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "external workspace resolver not registered")
	}

	return authMethod, resolver, nil
}

func newExternalIntegrationRequest(c *echo.Context, profile domain.ExternalIntegrationProfile) *port.ExternalIntegrationRequest {
	environment, _ := domain.ParseEnvironment(c.Request().Header.Get(ExternalIntegrationEnvironmentHeader))
	reqCtx := &port.ExternalIntegrationRequest{
		ProfileSlug: profile.Slug,
		Environment: environment,
		TenantCode:  c.Param("tenant_code"),
		Token:       strings.TrimSpace(c.Request().Header.Get(ExternalIntegrationTokenHeader)),
		Headers:     collectAllowedHeaders(c.Request().Header, profile.AllowedHeaders),
		QueryParams: copyQueryParams(c),
		Path:        c.Request().URL.Path,
		Method:      c.Request().Method,
	}
	if ws := c.Param("workspace_code"); ws != "" {
		reqCtx.WorkspaceCodes = []string{ws}
	}

	return reqCtx
}

func validateExternalTokenTransport(c *echo.Context) error {
	if token := strings.TrimSpace(c.QueryParam("token")); token != "" {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "external integration token must be provided via x-senda-external-token header")
	}

	if strings.TrimSpace(c.Request().Header.Get(ExternalIntegrationTokenHeader)) == "" {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing required header "+ExternalIntegrationTokenHeader)
	}

	return nil
}

func authenticateExternalRequest(c *echo.Context, authMethod port.ExternalAuthMethod, reqCtx *port.ExternalIntegrationRequest) (*port.ExternalAuthResult, error) {
	authResult, err := authMethod.Authenticate(c.Request().Context(), reqCtx)
	if err != nil {
		if errors.Is(err, errExternalAuthDenied) {
			return nil, response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "external integration access denied")
		}
		return nil, response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "external integration auth failed")
	}
	if authResult == nil {
		authResult = &port.ExternalAuthResult{}
	}

	return authResult, nil
}

func resolveExternalWorkspace(c *echo.Context, resolver port.ExternalWorkspaceResolver, reqCtx *port.ExternalIntegrationRequest, authResult *port.ExternalAuthResult, filter port.WorkspaceFilter) (*port.ExternalWorkspaceResolution, string, bool, error) {
	resolution, err := resolver.ResolveWorkspace(c.Request().Context(), reqCtx, authResult, filter)
	if err != nil || resolution == nil {
		return nil, "", false, response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "external workspace resolution failed")
	}

	readOnly := resolution.ReadOnly
	effectiveWorkspaceCode := resolution.WorkspaceCode
	if readOnly {
		effectiveWorkspaceCode = "_system"
	}
	if strings.TrimSpace(effectiveWorkspaceCode) == "" {
		return nil, "", false, response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "workspace resolution returned no workspace")
	}
	if effectiveWorkspaceCode == "_system" && !readOnly {
		return nil, "", false, response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "system workspace is read-only for this integration")
	}

	requestWorkspaceCode := c.Param("workspace_code")
	if !readOnly && requestWorkspaceCode != "" && requestWorkspaceCode != effectiveWorkspaceCode {
		return nil, "", false, response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "workspace resolution mismatch")
	}

	return resolution, effectiveWorkspaceCode, readOnly, nil
}

func applyExternalIntegrationContext(c *echo.Context, profile domain.ExternalIntegrationProfile, reqCtx *port.ExternalIntegrationRequest, authResult *port.ExternalAuthResult, resolution *port.ExternalWorkspaceResolution, effectiveWorkspaceCode string, readOnly bool) {
	effectivePermissions := combinePermissions(profile.Capabilities, authResult.Permissions)
	c.Set(ContextKeyExternalIntegrationProfile, profile)
	c.Set(ContextKeyExternalIntegrationAuthResult, authResult)
	c.Set(ContextKeyExternalIntegrationResolution, resolution)
	c.Set(ContextKeyExternalIntegrationPermissions, effectivePermissions)
	c.Set(ContextKeyExternalIntegrationReadOnly, readOnly)
	c.Set(ContextKeyExternalIntegrationEffectiveWorkspaceCode, effectiveWorkspaceCode)
	c.Set(ContextKeyExternalIntegrationAllowedHeaders, cloneStringMap(reqCtx.Headers))
	c.Set(ContextKeyEnvironment, reqCtx.Environment)
	c.Set(ContextKeyTenantCode, reqCtx.TenantCode)
	c.Set(ContextKeyWorkspaceCode, effectiveWorkspaceCode)

	patchExternalWorkspaceParam(c, effectiveWorkspaceCode)
}

func validateExternalEnvironment(c *echo.Context) error {
	environmentHeader := c.Request().Header.Get(ExternalIntegrationEnvironmentHeader)
	if strings.TrimSpace(environmentHeader) == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "missing required header X-Senda-Environment")
	}
	if _, err := domain.ParseEnvironment(environmentHeader); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid X-Senda-Environment header")
	}
	return nil
}

func mapExternalProfileLoadError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "external integration profile is disabled")
	case errors.Is(err, domain.ErrNotFound):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "external integration profile not found")
	default:
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load external integration profile")
	}
}

func responseCommitted(c *echo.Context) bool {
	resp, err := echo.UnwrapResponse(c.Response())
	if err != nil {
		return false
	}
	return resp.Committed
}

// ExternalIntegrationCORS enforces profile-scoped CORS for the external surface.
func ExternalIntegrationCORS(h ExternalIntegrationResolverStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			profileSlug := externalProfileSlugFromRequest(c)
			if profileSlug == "" {
				return next(c)
			}

			profile, err := h.LoadProfileBySlug(c.Request().Context(), profileSlug)
			if err != nil {
				switch {
				case errors.Is(err, domain.ErrForbidden):
					return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "external integration profile is disabled")
				case errors.Is(err, domain.ErrNotFound):
					return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "external integration profile not found")
				default:
					return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load external integration profile")
				}
			}

			origin := strings.TrimSpace(c.Request().Header.Get(echo.HeaderOrigin))
			if origin == "" {
				if c.Request().Method == http.MethodOptions {
					return c.NoContent(http.StatusNoContent)
				}
				return next(c)
			}

			if !originAllowed(origin, profile.AllowedOrigins) {
				return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "origin not allowed for this external integration profile")
			}

			applyExternalCORSHeaders(c, origin, profile.AllowedHeaders)
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}

func externalProfileSlugFromRequest(c *echo.Context) string {
	if slug := strings.TrimSpace(c.Param("profile_slug")); slug != "" {
		return slug
	}

	path := strings.TrimPrefix(c.Request().URL.Path, "/api/v1/external/")
	if path == c.Request().URL.Path {
		return ""
	}

	slug, _, _ := strings.Cut(path, "/")
	return strings.TrimSpace(slug)
}

// RequireExternalCapability enforces that the effective external permissions
// allow the requested action. Mutations are blocked outright in read-only mode.
func RequireExternalCapability(action ExternalAction) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			permissions := externalPermissions(c)
			if !hasExternalCapability(permissions, action) {
				return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "external capability not enabled")
			}

			return next(c)
		}
	}
}

// RequireExternalMutation blocks mutation routes when the resolver selected the
// read-only fallback.
func RequireExternalMutation() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if isExternalReadOnly(c) {
				return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "read-only integration cannot mutate resources")
			}
			return next(c)
		}
	}
}

func externalPermissions(c *echo.Context) port.ExternalPermissions {
	perms, _ := c.Get(ContextKeyExternalIntegrationPermissions).(port.ExternalPermissions)
	return perms
}

func isExternalReadOnly(c *echo.Context) bool {
	readOnly, _ := c.Get(ContextKeyExternalIntegrationReadOnly).(bool)
	return readOnly
}

func hasExternalCapability(perms port.ExternalPermissions, action ExternalAction) bool {
	switch action {
	case ExternalActionListTemplates:
		return perms.ListTemplates
	case ExternalActionViewVersions:
		return perms.ViewVersions
	case ExternalActionEditVersions:
		return perms.EditVersions
	case ExternalActionPublishVersions:
		return perms.PublishVersions
	case ExternalActionTestSend:
		return perms.TestSend
	case ExternalActionBuilderAccess:
		return perms.BuilderAccess
	case ExternalActionMetadataAccess:
		return perms.MetadataAccess
	case ExternalActionLocaleAccess:
		return perms.LocaleAccess
	default:
		return false
	}
}

func combinePermissions(profile domain.ExternalIntegrationCapabilities, auth port.ExternalPermissions) port.ExternalPermissions {
	return port.ExternalPermissions{
		ListTemplates:   profile.ListTemplates && auth.ListTemplates,
		ViewVersions:    profile.ViewVersions && auth.ViewVersions,
		EditVersions:    profile.EditVersions && auth.EditVersions,
		PublishVersions: profile.PublishVersions && auth.PublishVersions,
		TestSend:        profile.TestSend && auth.TestSend,
		BuilderAccess:   profile.BuilderAccess && auth.BuilderAccess,
		MetadataAccess:  profile.MetadataAccess && auth.MetadataAccess,
		LocaleAccess:    profile.LocaleAccess && auth.LocaleAccess,
	}
}

func collectAllowedHeaders(headers http.Header, allowed []string) map[string]string {
	if len(allowed) == 0 {
		return nil
	}

	out := make(map[string]string, len(allowed))
	for _, header := range allowed {
		if value := headers.Get(header); value != "" {
			out[header] = value
		}
	}
	return out
}

func copyQueryParams(c *echo.Context) map[string]string {
	query := c.QueryParams()
	if len(query) == 0 {
		return nil
	}

	out := make(map[string]string, len(query))
	for key, values := range query {
		if len(values) == 0 {
			continue
		}
		out[key] = values[0]
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), origin) {
			return true
		}
	}
	return false
}

func applyExternalCORSHeaders(c *echo.Context, origin string, allowedHeaders []string) {
	header := c.Response().Header()
	header.Set(echo.HeaderAccessControlAllowOrigin, origin)
	header.Add(echo.HeaderVary, echo.HeaderOrigin)
	header.Set(echo.HeaderAccessControlAllowMethods, strings.Join([]string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
	}, ", "))
	header.Set(echo.HeaderAccessControlAllowHeaders, strings.Join(externalCORSAllowedHeaders(allowedHeaders), ", "))
	header.Set(echo.HeaderAccessControlExposeHeaders, "X-Request-ID")
}

func externalCORSAllowedHeaders(allowedHeaders []string) []string {
	base := []string{
		echo.HeaderContentType,
		"X-Request-ID",
		"X-Senda-External-Token",
	}

	seen := make(map[string]struct{}, len(base)+len(allowedHeaders))
	out := make([]string, 0, len(base)+len(allowedHeaders))
	for _, header := range append(base, allowedHeaders...) {
		normalized := http.CanonicalHeaderKey(strings.TrimSpace(header))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func ExternalAllowedHeaders(c *echo.Context) map[string]string {
	headers, _ := c.Get(ContextKeyExternalIntegrationAllowedHeaders).(map[string]string)
	return cloneStringMap(headers)
}

func patchExternalWorkspaceParam(c *echo.Context, workspaceCode string) {
	paths := c.PathValues()
	updated := make(echo.PathValues, 0, len(paths))
	for _, pv := range paths {
		if pv.Name == "workspace_code" {
			pv.Value = workspaceCode
		}
		updated = append(updated, pv)
	}
	c.SetPathValues(updated)
}

// buildWorkspaceFilter constructs a per-request WorkspaceFilter bound to the
// tenant and environment from reqCtx. Returns nil when store is nil so that
// the resolved filter propagates as a nil port.WorkspaceFilter to the resolver.
func buildWorkspaceFilter(store port.WorkspaceExistenceStore, reqCtx *port.ExternalIntegrationRequest) port.WorkspaceFilter {
	if store == nil {
		return nil
	}
	return newWorkspaceFilter(store, reqCtx.TenantCode, reqCtx.Environment)
}

// ExternalAuthDenied returns a sentinel error that external auth methods can
// use to signal a hard deny instead of an infrastructure failure.
func ExternalAuthDenied() error {
	return errExternalAuthDenied
}

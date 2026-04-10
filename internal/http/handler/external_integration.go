package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// ExternalIntegrationHandler exposes the bootstrap surface for
// embeddable external integrations.
type ExternalIntegrationHandler struct {
	store               port.GlobalConfigStore
	externalAuthMethods []port.ExternalAuthMethod
	externalResolvers   []port.ExternalWorkspaceResolver
}

// NewExternalIntegrationHandler creates a new handler for the external surface.
func NewExternalIntegrationHandler(
	store port.GlobalConfigStore,
	authMethods []port.ExternalAuthMethod,
	resolvers []port.ExternalWorkspaceResolver,
) *ExternalIntegrationHandler {
	return &ExternalIntegrationHandler{
		store:               store,
		externalAuthMethods: authMethods,
		externalResolvers:   resolvers,
	}
}

// Bootstrap handles GET /api/v1/external/:profile_slug/bootstrap.
func (h *ExternalIntegrationHandler) Bootstrap(c *echo.Context) error {
	profile, _, err := h.loadProfile(c)
	if err != nil {
		return err
	}

	frameAncestors := frameAncestorsFromProfile(profile.AllowedOrigins)
	applyExternalFrameHeaders(c, frameAncestors)

	return c.JSON(http.StatusOK, response.NewExternalIntegrationBootstrapResponse(frameAncestors))
}

// Session exposes the authenticated external embed runtime state so the UI can
// render the effective permissions and fallback mode without relying on
// management-only endpoints.
func (h *ExternalIntegrationHandler) Session(c *echo.Context) error {
	perms, _ := c.Get(middleware.ContextKeyExternalIntegrationPermissions).(port.ExternalPermissions)
	readOnly, _ := c.Get(middleware.ContextKeyExternalIntegrationReadOnly).(bool)
	effectiveWorkspaceCode, _ := c.Get(middleware.ContextKeyExternalIntegrationEffectiveWorkspaceCode).(string)

	return c.JSON(http.StatusOK, response.ExternalIntegrationSessionResponse{
		ReadOnly:               readOnly,
		EffectiveWorkspaceCode: effectiveWorkspaceCode,
		Permissions: response.ExternalIntegrationCapabilitiesResponse{
			ListTemplates:   perms.ListTemplates,
			ViewVersions:    perms.ViewVersions,
			EditVersions:    perms.EditVersions,
			PublishVersions: perms.PublishVersions,
			TestSend:        perms.TestSend,
			BuilderAccess:   perms.BuilderAccess,
			MetadataAccess:  perms.MetadataAccess,
			LocaleAccess:    perms.LocaleAccess,
		},
	})
}

// LoadProfileBySlug returns an enabled external integration profile by slug.
func (h *ExternalIntegrationHandler) LoadProfileBySlug(ctx context.Context, slug string) (domain.ExternalIntegrationProfile, error) {
	cfg, err := h.store.Get(ctx)
	if err != nil {
		return domain.ExternalIntegrationProfile{}, err
	}

	slug = strings.TrimSpace(strings.ToLower(slug))
	for _, profile := range cfg.ExternalIntegrations {
		if profile.Slug == slug {
			if !profile.Enabled {
				return domain.ExternalIntegrationProfile{}, domain.ErrForbidden
			}
			return profile, nil
		}
	}

	return domain.ExternalIntegrationProfile{}, domain.ErrNotFound
}

// AuthMethodByName returns a registered auth method by name.
func (h *ExternalIntegrationHandler) AuthMethodByName(name string) (port.ExternalAuthMethod, bool) {
	for _, method := range h.externalAuthMethods {
		if method != nil && method.Name() == name {
			return method, true
		}
	}
	return nil, false
}

// ResolverByName returns a registered workspace resolver by name.
func (h *ExternalIntegrationHandler) ResolverByName(name string) (port.ExternalWorkspaceResolver, bool) {
	for _, resolver := range h.externalResolvers {
		if resolver != nil && resolver.Name() == name {
			return resolver, true
		}
	}
	return nil, false
}

func (h *ExternalIntegrationHandler) loadProfile(c *echo.Context) (domain.ExternalIntegrationProfile, response.ExternalIntegrationProfileResponse, error) {
	profile, err := h.LoadProfileBySlug(c.Request().Context(), c.Param("profile_slug"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return domain.ExternalIntegrationProfile{}, response.ExternalIntegrationProfileResponse{}, response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "external integration profile is disabled")
		case errors.Is(err, domain.ErrNotFound):
			return domain.ExternalIntegrationProfile{}, response.ExternalIntegrationProfileResponse{}, response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "external integration profile not found")
		default:
			return domain.ExternalIntegrationProfile{}, response.ExternalIntegrationProfileResponse{}, mapStoreError(c, err)
		}
	}

	return profile, toExternalIntegrationProfileResponse(profile), nil
}

func toExternalIntegrationProfileResponse(profile domain.ExternalIntegrationProfile) response.ExternalIntegrationProfileResponse {
	return response.ExternalIntegrationProfileResponse{
		Slug:            profile.Slug,
		Name:            profile.Name,
		Description:     profile.Description,
		Enabled:         profile.Enabled,
		AuthMethodName:  profile.AuthMethodName,
		ResolverName:    profile.ResolverName,
		AllowedOrigins:  append([]string(nil), profile.AllowedOrigins...),
		AllowedHeaders:  append([]string(nil), profile.AllowedHeaders...),
		RequiredHeaders: append([]string(nil), profile.RequiredHeaders...),
		Capabilities: response.ExternalIntegrationCapabilitiesResponse{
			ListTemplates:   profile.Capabilities.ListTemplates,
			ViewVersions:    profile.Capabilities.ViewVersions,
			EditVersions:    profile.Capabilities.EditVersions,
			PublishVersions: profile.Capabilities.PublishVersions,
			TestSend:        profile.Capabilities.TestSend,
			BuilderAccess:   profile.Capabilities.BuilderAccess,
			MetadataAccess:  profile.Capabilities.MetadataAccess,
			LocaleAccess:    profile.Capabilities.LocaleAccess,
		},
	}
}

func frameAncestorsFromProfile(origins []string) []string {
	if len(origins) == 0 {
		return []string{"'none'"}
	}

	out := make([]string, 0, len(origins)+1)
	out = append(out, "'self'")
	out = append(out, origins...)
	return out
}

func applyExternalFrameHeaders(c *echo.Context, frameAncestors []string) {
	header := c.Response().Header()
	header.Del("X-Frame-Options")
	header.Set("Content-Security-Policy", "frame-ancestors "+strings.Join(frameAncestors, " "))
}

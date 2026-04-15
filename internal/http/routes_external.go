package http

import (
	"github.com/rendis/senda/internal/http/middleware"
)

func (s *Server) registerExternalRoutes() {
	if s.externalIntegrationHandler == nil {
		return
	}

	external := s.echo.Group("/api/v1/external/:profile_slug")
	external.GET("/bootstrap", s.externalIntegrationHandler.Bootstrap)
	external.GET("/environments/:environment/bootstrap", s.externalIntegrationHandler.Bootstrap)

	if s.templateTypeHandler == nil && s.templateHandler == nil && s.injectorHandler == nil && s.workspacePolicyHandler == nil {
		return
	}

	externalScoped := external.Group("/tenants/:tenant_code/workspaces/:workspace_code")
	externalScoped.Use(middleware.ExternalIntegration(s.externalIntegrationHandler))
	externalScoped.GET("/session", s.externalIntegrationHandler.Session, middleware.RequireExternalCapability(middleware.ExternalActionBuilderAccess))

	if s.templateTypeHandler != nil {
		externalScoped.GET("/template-types", s.templateTypeHandler.List, middleware.RequireExternalCapability(middleware.ExternalActionListTemplates))
		externalScoped.GET("/template-types/:slug", s.templateTypeHandler.Get, middleware.RequireExternalCapability(middleware.ExternalActionListTemplates))
	}

	if s.templateHandler != nil {
		externalScoped.GET("/template-types/:slug/templates", s.templateHandler.ListByTemplateType, middleware.RequireExternalCapability(middleware.ExternalActionListTemplates))
		externalScoped.GET("/templates/:template_id/versions", s.templateHandler.ListVersions, middleware.RequireExternalCapability(middleware.ExternalActionViewVersions))
		externalScoped.GET("/templates/:template_id/versions/:version_id", s.templateHandler.GetVersion, middleware.RequireExternalCapability(middleware.ExternalActionViewVersions))
		externalScoped.PUT("/templates/:template_id/versions/:version_id", s.templateHandler.UpdateVersion, middleware.RequireExternalCapability(middleware.ExternalActionEditVersions), middleware.RequireExternalMutation())
		externalScoped.POST("/templates/:template_id/versions/:version_id/publish", s.templateHandler.PublishVersion, middleware.RequireExternalCapability(middleware.ExternalActionPublishVersions), middleware.RequireExternalMutation())
		externalScoped.GET("/templates/:template_id/versions/:version_id/locales", s.templateHandler.ListLocales, middleware.RequireExternalCapability(middleware.ExternalActionLocaleAccess))
		externalScoped.GET("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.GetLocale, middleware.RequireExternalCapability(middleware.ExternalActionLocaleAccess))
		externalScoped.POST("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.SetLocale, middleware.RequireExternalCapability(middleware.ExternalActionLocaleAccess), middleware.RequireExternalMutation())
		externalScoped.PUT("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.UpdateLocale, middleware.RequireExternalCapability(middleware.ExternalActionLocaleAccess), middleware.RequireExternalMutation())
		externalScoped.DELETE("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.DeleteLocale, middleware.RequireExternalCapability(middleware.ExternalActionLocaleAccess), middleware.RequireExternalMutation())
		externalScoped.POST("/templates/:template_id/preview-mjml", s.templateHandler.PreviewMJML, middleware.RequireExternalCapability(middleware.ExternalActionBuilderAccess))
		externalScoped.POST("/templates/:template_id/test-send", s.templateHandler.TestSend, middleware.RequireExternalCapability(middleware.ExternalActionTestSend), middleware.RequireExternalMutation())
	}

	if s.injectorHandler != nil {
		externalScoped.GET("/injectors", s.injectorHandler.List, middleware.RequireExternalCapability(middleware.ExternalActionBuilderAccess))
		externalScoped.GET("/injectors/:name", s.injectorHandler.Get, middleware.RequireExternalCapability(middleware.ExternalActionBuilderAccess))
	}

	if s.workspacePolicyHandler != nil {
		externalScoped.GET("/policies", s.workspacePolicyHandler.Get, middleware.RequireExternalCapability(middleware.ExternalActionBuilderAccess))
	}
}

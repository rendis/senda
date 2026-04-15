package http

import (
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
)

func (s *Server) registerManagementRoutes() {
	api := s.echo.Group("/api/v1")
	auth := middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper)

	if s.memberHandler != nil {
		api.GET("/members/me", s.memberHandler.Me, auth, middleware.OIDCOnly())
	}

	if !s.hasManagementSurface() {
		return
	}

	mgmt := s.echo.Group("/api/v1/manage")
	mgmt.Use(auth)
	mgmt.Use(middleware.OIDCOnly())

	if s.tenantHandler != nil {
		mgmt.POST("/tenants", s.tenantHandler.Create, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants", s.tenantHandler.List, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants/:tenant_code", s.tenantHandler.GetByCode, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.PUT("/tenants/:tenant_code", s.tenantHandler.Update, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.DELETE("/tenants/:tenant_code", s.tenantHandler.SoftDelete, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
	}

	if s.workspaceHandler != nil {
		mgmt.POST("/tenants/:tenant_code/workspaces", s.workspaceHandler.Create, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants/:tenant_code/workspaces", s.workspaceHandler.List, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		mgmt.PUT("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		mgmt.DELETE("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.SoftDelete, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.GET("/environments/:environment/tenants/:tenant_code/workspaces", s.workspaceHandler.List, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
	}

	if s.workspacePolicyHandler != nil {
		mgmt.GET("/tenants/:tenant_code/workspaces/:workspace_code/policies", s.workspacePolicyHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		mgmt.PUT("/tenants/:tenant_code/workspaces/:workspace_code/policies", s.workspacePolicyHandler.Update, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
	}

	if s.dashboardHandler != nil {
		mgmt.GET("/tenants/:tenant_code/dashboard-stats", s.dashboardHandler.StatsTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
	}

	if s.memberHandler != nil {
		mgmt.GET("/members", s.memberHandler.List, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.POST("/members", s.memberHandler.Create, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.GET("/members/:member_id", s.memberHandler.Get, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.DELETE("/members/:member_id/access", s.memberHandler.RemoveAccess, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.PUT("/members/:member_id/role", s.memberHandler.ReplaceRole, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.POST("/members/:member_id/roles", s.memberHandler.AddRole, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.DELETE("/members/:member_id/roles/:role_id", s.memberHandler.RemoveRole, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))

		tenantMembers := mgmt.Group("/tenants/:tenant_code")
		tenantMembers.GET("/members", s.memberHandler.ListTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		tenantMembers.POST("/members", s.memberHandler.Create, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		tenantMembers.GET("/members/:member_id", s.memberHandler.GetTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		tenantMembers.DELETE("/members/:member_id/access", s.memberHandler.RemoveAccessTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		tenantMembers.PUT("/members/:member_id/role", s.memberHandler.ReplaceRoleTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		tenantMembers.POST("/members/:member_id/roles", s.memberHandler.AddRoleTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		tenantMembers.DELETE("/members/:member_id/roles/:role_id", s.memberHandler.RemoveRoleTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
	}

	if s.configHandler != nil {
		mgmt.GET("/config", s.configHandler.Get, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.PUT("/config", s.configHandler.Update, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
	}

	ws := mgmt.Group("/tenants/:tenant_code/workspaces/:workspace_code")
	s.registerManagementWorkspaceRoutes(ws)

	envWS := mgmt.Group("/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code")
	if s.workspaceHandler != nil {
		envWS.GET("", s.workspaceHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		envWS.PUT("", s.workspaceHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		envWS.POST("/runtime/reset", s.workspaceHandler.ResetRuntime, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}
	if s.workspacePolicyHandler != nil {
		envWS.GET("/policies", s.workspacePolicyHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		envWS.PUT("/policies", s.workspacePolicyHandler.Update, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
	}
	s.registerManagementWorkspaceRoutes(envWS)

	global := mgmt.Group("/global", middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
	s.registerManagementGlobalRoutes(global)
}

func (s *Server) hasManagementSurface() bool {
	return s.tenantHandler != nil ||
		s.workspaceHandler != nil ||
		s.workspacePolicyHandler != nil ||
		s.memberHandler != nil ||
		s.configHandler != nil ||
		s.injectorHandler != nil ||
		s.adapterHandler != nil ||
		s.identityHandler != nil ||
		s.adapterSetupHandler != nil ||
		s.templateTypeHandler != nil ||
		s.templateHandler != nil ||
		s.emailHandler != nil ||
		s.suppressionHandler != nil ||
		s.auditHandler != nil ||
		s.webhookHandler != nil ||
		s.apiKeyHandler != nil ||
		s.dashboardHandler != nil
}

func (s *Server) registerManagementWorkspaceRoutes(ws *echo.Group) {
	if s.injectorHandler != nil {
		ws.POST("/injectors", s.injectorHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/injectors", s.injectorHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.GET("/injectors/:name", s.injectorHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.PUT("/injectors/:name", s.injectorHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.PUT("/injectors/:name/fields/:field_name", s.injectorHandler.UpdateField, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.PUT("/injectors/:name/values", s.injectorHandler.SetValues, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.DELETE("/injectors/:name", s.injectorHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.adapterHandler != nil {
		ws.GET("/adapters", s.adapterHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/adapters", s.adapterHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/adapters/validate-ses", s.adapterHandler.ValidateSES, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/adapters/:id", s.adapterHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.PUT("/adapters/:id", s.adapterHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/adapters/:id", s.adapterHandler.SoftDelete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/adapters/:id/test", s.adapterHandler.TestConnection, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.GET("/adapters/:id/workspace-access", s.adapterHandler.GetWorkspaceAccess, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.PUT("/adapters/:id/workspace-access", s.adapterHandler.UpdateWorkspaceAccess, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		if s.adapterSetupHandler != nil {
			ws.GET("/adapters/:id/setup-guide", s.adapterSetupHandler.SetupGuide, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
			ws.POST("/adapters/:id/auto-provision-tracking", s.adapterSetupHandler.AutoProvision, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			ws.GET("/adapters/:id/provisioning-status", s.adapterSetupHandler.ProvisioningStatus, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		}
	}

	if s.identityHandler != nil {
		ws.GET("/adapters/:id/identities", s.identityHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/adapters/:id/identities", s.identityHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/adapters/:id/identities/sync", s.identityHandler.Sync, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/adapters/:id/identities/:identity_id", s.identityHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/adapters/:id/identities/:identity_id/set-default", s.identityHandler.SetDefault, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/adapters/:id/identities/:identity_id/workspace-access", s.identityHandler.GetWorkspaceAccess, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.PUT("/adapters/:id/identities/:identity_id/workspace-access", s.identityHandler.UpdateWorkspaceAccess, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.templateTypeHandler != nil {
		ws.POST("/template-types", s.templateTypeHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/template-types", s.templateTypeHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.GET("/template-types/:slug", s.templateTypeHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.PUT("/template-types/:slug", s.templateTypeHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/template-types/:slug", s.templateTypeHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.templateHandler != nil {
		ws.GET("/template-types/:slug/templates", s.templateHandler.ListByTemplateType, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/templates", s.templateHandler.CreateTemplate, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/fork", s.templateHandler.ForkTemplate, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.GET("/templates/:template_id/versions", s.templateHandler.ListVersions, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.GET("/templates/:template_id/versions/:version_id", s.templateHandler.GetVersion, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/versions", s.templateHandler.CreateVersion, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.PUT("/templates/:template_id/versions/:version_id", s.templateHandler.UpdateVersion, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/versions/:version_id/clone", s.templateHandler.CloneVersion, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/versions/:version_id/publish", s.templateHandler.PublishVersion, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/templates/:template_id/versions/:version_id/locales", s.templateHandler.ListLocales, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.SetLocale, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.PUT("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.UpdateLocale, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.GET("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.GetLocale, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.DELETE("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.DeleteLocale, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/preview-mjml", s.templateHandler.PreviewMJML, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/test-send", s.templateHandler.TestSend, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.GET("/templates/:template_id/bulk-send-config", s.templateHandler.BulkSendConfig, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/bulk-send", s.templateHandler.BulkSend, middleware.RequireRole(domain.RoleWorkspaceEditor, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/disable", s.templateHandler.DisableTemplate, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/templates/:template_id/enable", s.templateHandler.EnableTemplate, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/templates/:template_id", s.templateHandler.DeleteTemplate, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/templates/:template_id/versions/:version_id", s.templateHandler.DeleteVersion, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.apiKeyHandler != nil {
		ws.POST("/api-keys", s.apiKeyHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/api-keys", s.apiKeyHandler.List, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/api-keys/:id", s.apiKeyHandler.Revoke, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.webhookHandler != nil {
		ws.POST("/webhooks", s.webhookHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/webhooks", s.webhookHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.GET("/webhooks/:id", s.webhookHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.PUT("/webhooks/:id", s.webhookHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/webhooks/:id", s.webhookHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/webhooks/:id/test", s.webhookHandler.Test, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.emailHandler != nil {
		ws.GET("/emails", s.emailHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.GET("/emails/:tracking_id", s.emailHandler.GetByTrackingID, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.GET("/emails/:tracking_id/events", s.emailHandler.GetEvents, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
	}

	if s.memberHandler != nil {
		ws.GET("/members", s.memberHandler.ListWorkspace, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.POST("/members", s.memberHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/members/:member_id", s.memberHandler.GetWorkspace, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.DELETE("/members/:member_id/access", s.memberHandler.RemoveAccessWorkspace, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.PUT("/members/:member_id/role", s.memberHandler.ReplaceRoleWorkspace, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.POST("/members/:member_id/roles", s.memberHandler.AddRoleWorkspace, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.DELETE("/members/:member_id/roles/:role_id", s.memberHandler.RemoveRoleWorkspace, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.suppressionHandler != nil {
		ws.POST("/suppression", s.suppressionHandler.Add, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
		ws.GET("/suppression/:email", s.suppressionHandler.Check, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
		ws.DELETE("/suppression/:email", s.suppressionHandler.Remove, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
	}

	if s.auditHandler != nil {
		ws.GET("/audit-log", s.auditHandler.Query, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
	}

	if s.dashboardHandler != nil {
		ws.GET("/dashboard-stats", s.dashboardHandler.Stats, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
	}
}

func (s *Server) registerManagementGlobalRoutes(global *echo.Group) {
	if s.injectorHandler != nil {
		global.POST("/injectors", s.injectorHandler.CreateGlobal)
		global.GET("/injectors", s.injectorHandler.ListGlobal)
		global.GET("/injectors/:name", s.injectorHandler.GetGlobal)
		global.PUT("/injectors/:name", s.injectorHandler.UpdateGlobal)
		global.PUT("/injectors/:name/fields/:field_name", s.injectorHandler.UpdateFieldGlobal)
		global.DELETE("/injectors/:name", s.injectorHandler.DeleteGlobal)
	}

	if s.adapterHandler != nil {
		global.GET("/adapters", s.adapterHandler.ListGlobal)
		global.POST("/adapters", s.adapterHandler.CreateGlobal)
		global.GET("/adapters/:id", s.adapterHandler.GetGlobal)
		global.PUT("/adapters/:id", s.adapterHandler.UpdateGlobal)
		global.DELETE("/adapters/:id", s.adapterHandler.SoftDeleteGlobal)
		global.POST("/adapters/:id/test", s.adapterHandler.TestConnectionGlobal)
		if s.adapterSetupHandler != nil {
			global.GET("/adapters/:id/setup-guide", s.adapterSetupHandler.SetupGuideGlobal)
			global.POST("/adapters/:id/auto-provision-tracking", s.adapterSetupHandler.AutoProvisionGlobal)
			global.GET("/adapters/:id/provisioning-status", s.adapterSetupHandler.ProvisioningStatusGlobal)
		}
	}

	if s.identityHandler != nil {
		global.GET("/adapters/:id/identities", s.identityHandler.ListGlobal)
		global.POST("/adapters/:id/identities", s.identityHandler.CreateGlobal)
		global.POST("/adapters/:id/identities/sync", s.identityHandler.SyncGlobal)
		global.DELETE("/adapters/:id/identities/:identity_id", s.identityHandler.DeleteGlobal)
		global.POST("/adapters/:id/identities/:identity_id/set-default", s.identityHandler.SetDefaultGlobal)
	}

	if s.templateTypeHandler != nil {
		global.POST("/template-types", s.templateTypeHandler.CreateGlobal)
		global.GET("/template-types", s.templateTypeHandler.ListGlobal)
		global.GET("/template-types/:slug", s.templateTypeHandler.GetGlobal)
		global.PUT("/template-types/:slug", s.templateTypeHandler.UpdateGlobal)
		global.DELETE("/template-types/:slug", s.templateTypeHandler.DeleteGlobal)
	}

	if s.templateHandler != nil {
		global.GET("/template-types/:slug/templates", s.templateHandler.ListByTemplateTypeGlobal)
		global.POST("/templates", s.templateHandler.CreateTemplateGlobal)
		global.GET("/templates/:template_id/versions", s.templateHandler.ListVersions)
		global.GET("/templates/:template_id/versions/:version_id", s.templateHandler.GetVersion)
		global.POST("/templates/:template_id/versions", s.templateHandler.CreateVersion)
		global.PUT("/templates/:template_id/versions/:version_id", s.templateHandler.UpdateVersion)
		global.POST("/templates/:template_id/versions/:version_id/clone", s.templateHandler.CloneVersionGlobal)
		global.POST("/templates/:template_id/versions/:version_id/publish", s.templateHandler.PublishVersion)
		global.GET("/templates/:template_id/versions/:version_id/locales", s.templateHandler.ListLocales)
		global.POST("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.SetLocale)
		global.PUT("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.UpdateLocale)
		global.GET("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.GetLocale)
		global.DELETE("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.DeleteLocale)
		global.POST("/templates/:template_id/preview-mjml", s.templateHandler.PreviewMJML)
		global.POST("/templates/:template_id/test-send", s.templateHandler.TestSend)
		global.POST("/templates/:template_id/disable", s.templateHandler.DisableTemplateGlobal)
		global.POST("/templates/:template_id/enable", s.templateHandler.EnableTemplateGlobal)
		global.DELETE("/templates/:template_id", s.templateHandler.DeleteTemplate)
		global.DELETE("/templates/:template_id/versions/:version_id", s.templateHandler.DeleteVersion)
	}

	if s.auditHandler != nil {
		global.GET("/audit-log", s.auditHandler.QueryGlobal)
	}

	if s.dashboardHandler != nil {
		global.GET("/dashboard-stats", s.dashboardHandler.StatsGlobal)
	}
}

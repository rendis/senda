package http

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// Server wraps the Echo instance with application configuration and logger.
type Server struct {
	echo   *echo.Echo
	config *config.Config
	logger *slog.Logger
	pinger handler.Pinger

	// Auth dependencies.
	apiKeyStore  port.APIKeyStore
	memberStore  port.MemberStore
	oidcVerifier port.OIDCVerifier
	apiKeyPepper string // HMAC pepper for API key hashing (derived from master key)

	// Store dependencies for RBAC middleware.
	tenantStore port.TenantStore
	wsStore     port.WorkspaceStore
	configStore port.GlobalConfigStore

	// Handlers.
	tenantHandler          *handler.TenantHandler
	workspaceHandler       *handler.WorkspaceHandler
	workspacePolicyHandler *handler.WorkspacePolicyHandler
	memberHandler          *handler.MemberHandler
	configHandler          *handler.ConfigHandler

	// HT-20 handlers.
	injectorHandler *handler.InjectorHandler
	adapterHandler  *handler.AdapterHandler

	// Identity handler (adapter identities).
	identityHandler *handler.IdentityHandler

	// Adapter setup guide handler.
	adapterSetupHandler *handler.AdapterSetupHandler

	// HT-21 handlers (Templates).
	templateTypeHandler *handler.TemplateTypeHandler
	templateHandler     *handler.TemplateHandler

	// HT-22 handlers (Send + Email Query).
	sendHandler           *handler.SendHandler
	emailHandler          *handler.EmailHandler
	dataPlaneEmailHandler *handler.DataPlaneEmailHandler
	suppressionHandler    *handler.SuppressionHandler
	auditHandler          *handler.AuditHandler

	// HT-23 handler (SES Webhooks).
	sesWebhookHandler *handler.SESWebhookHandler

	// HT-24 handler (Webhooks CRUD).
	webhookHandler *handler.WebhookHandler

	// HT-25 handler (Onboarding).
	onboardingHandler *handler.OnboardingHandler

	// HT-27 handler (API Keys).
	apiKeyHandler *handler.APIKeyHandler

	// Dashboard handler.
	dashboardHandler *handler.DashboardHandler

	// Open-tracking handler (public, no auth).
	trackingHandler *handler.TrackingHandler

	// Media handler (public, no auth).
	mediaHandler *handler.MediaHandler
}

// ServerOption configures optional Server dependencies.
type ServerOption func(*Server)

// WithPinger sets the Pinger used by the health endpoint to check DB connectivity.
func WithPinger(p handler.Pinger) ServerOption {
	return func(s *Server) {
		s.pinger = p
	}
}

// WithAuthDeps sets authentication dependencies for the Auth middleware.
// The pepper parameter is the HMAC pepper derived from the master key for API key hashing.
func WithAuthDeps(apiKeyStore port.APIKeyStore, memberStore port.MemberStore, oidcVerifier port.OIDCVerifier, pepper string) ServerOption {
	return func(s *Server) {
		s.apiKeyStore = apiKeyStore
		s.memberStore = memberStore
		s.oidcVerifier = oidcVerifier
		s.apiKeyPepper = pepper
	}
}

// WithTenantStore sets the TenantStore for RBAC middleware route resolution.
func WithTenantStore(ts port.TenantStore) ServerOption {
	return func(s *Server) {
		s.tenantStore = ts
	}
}

// WithWorkspaceStore sets the WorkspaceStore for RBAC middleware route resolution.
func WithWorkspaceStore(ws port.WorkspaceStore) ServerOption {
	return func(s *Server) {
		s.wsStore = ws
	}
}

// WithConfigStore sets the GlobalConfigStore for the config handler.
func WithConfigStore(cs port.GlobalConfigStore) ServerOption {
	return func(s *Server) {
		s.configStore = cs
	}
}

// WithTenantHandler sets the TenantHandler for tenant CRUD routes.
func WithTenantHandler(h *handler.TenantHandler) ServerOption {
	return func(s *Server) {
		s.tenantHandler = h
	}
}

// WithWorkspaceHandler sets the WorkspaceHandler for workspace CRUD routes.
func WithWorkspaceHandler(h *handler.WorkspaceHandler) ServerOption {
	return func(s *Server) {
		s.workspaceHandler = h
	}
}

// WithWorkspacePolicyHandler sets the WorkspacePolicyHandler for _system policy routes.
func WithWorkspacePolicyHandler(h *handler.WorkspacePolicyHandler) ServerOption {
	return func(s *Server) {
		s.workspacePolicyHandler = h
	}
}

// WithMemberHandler sets the MemberHandler for member CRUD routes.
func WithMemberHandler(h *handler.MemberHandler) ServerOption {
	return func(s *Server) {
		s.memberHandler = h
	}
}

// WithConfigHandler sets the ConfigHandler for config CRUD routes.
func WithConfigHandler(h *handler.ConfigHandler) ServerOption {
	return func(s *Server) {
		s.configHandler = h
	}
}

// WithInjectorHandler sets the InjectorHandler for injector CRUD routes.
func WithInjectorHandler(h *handler.InjectorHandler) ServerOption {
	return func(s *Server) {
		s.injectorHandler = h
	}
}

// WithAdapterHandler sets the AdapterHandler for adapter CRUD routes.
func WithAdapterHandler(h *handler.AdapterHandler) ServerOption {
	return func(s *Server) {
		s.adapterHandler = h
	}
}

// WithIdentityHandler sets the IdentityHandler for adapter identity routes.
func WithIdentityHandler(h *handler.IdentityHandler) ServerOption {
	return func(s *Server) {
		s.identityHandler = h
	}
}

// WithAdapterSetupHandler sets the AdapterSetupHandler for setup guide routes.
func WithAdapterSetupHandler(h *handler.AdapterSetupHandler) ServerOption {
	return func(s *Server) {
		s.adapterSetupHandler = h
	}
}

// WithTemplateTypeHandler sets the TemplateTypeHandler for template type CRUD routes.
func WithTemplateTypeHandler(h *handler.TemplateTypeHandler) ServerOption {
	return func(s *Server) {
		s.templateTypeHandler = h
	}
}

// WithTemplateHandler sets the TemplateHandler for template/version/locale CRUD routes.
func WithTemplateHandler(h *handler.TemplateHandler) ServerOption {
	return func(s *Server) {
		s.templateHandler = h
	}
}

// WithSendHandler sets the SendHandler for the data-plane send endpoint.
func WithSendHandler(h *handler.SendHandler) ServerOption {
	return func(s *Server) {
		s.sendHandler = h
	}
}

// WithEmailHandler sets the EmailHandler for email query endpoints.
func WithEmailHandler(h *handler.EmailHandler) ServerOption {
	return func(s *Server) {
		s.emailHandler = h
	}
}

// WithDataPlaneEmailHandler sets the API-key-scoped data-plane email query handler.
func WithDataPlaneEmailHandler(h *handler.DataPlaneEmailHandler) ServerOption {
	return func(s *Server) {
		s.dataPlaneEmailHandler = h
	}
}

// WithSuppressionHandler sets the SuppressionHandler for suppression list management.
func WithSuppressionHandler(h *handler.SuppressionHandler) ServerOption {
	return func(s *Server) {
		s.suppressionHandler = h
	}
}

// WithAuditHandler sets the AuditHandler for audit log queries.
func WithAuditHandler(h *handler.AuditHandler) ServerOption {
	return func(s *Server) {
		s.auditHandler = h
	}
}

// WithSESWebhookHandler sets the SES webhook handler for provider event ingestion.
func WithSESWebhookHandler(h *handler.SESWebhookHandler) ServerOption {
	return func(s *Server) {
		s.sesWebhookHandler = h
	}
}

// WithWebhookHandler sets the WebhookHandler for webhook CRUD routes.
func WithWebhookHandler(h *handler.WebhookHandler) ServerOption {
	return func(s *Server) {
		s.webhookHandler = h
	}
}

// WithOnboardingHandler sets the OnboardingHandler for onboarding routes.
func WithOnboardingHandler(h *handler.OnboardingHandler) ServerOption {
	return func(s *Server) {
		s.onboardingHandler = h
	}
}

// WithAPIKeyHandler sets the APIKeyHandler for API key management routes.
func WithAPIKeyHandler(h *handler.APIKeyHandler) ServerOption {
	return func(s *Server) {
		s.apiKeyHandler = h
	}
}

// WithDashboardHandler sets the DashboardHandler for dashboard stats routes.
func WithDashboardHandler(h *handler.DashboardHandler) ServerOption {
	return func(s *Server) {
		s.dashboardHandler = h
	}
}

// WithTrackingHandler sets the TrackingHandler for open-tracking pixel routes.
func WithTrackingHandler(h *handler.TrackingHandler) ServerOption {
	return func(s *Server) {
		s.trackingHandler = h
	}
}

// WithMediaHandler sets the MediaHandler for public media utility routes.
func WithMediaHandler(h *handler.MediaHandler) ServerOption {
	return func(s *Server) {
		s.mediaHandler = h
	}
}

// NewServer creates a configured Echo server with middleware and routes.
func NewServer(cfg *config.Config, logger *slog.Logger, opts ...ServerOption) *Server {
	e := echo.New()

	e.HTTPErrorHandler = response.HTTPErrorHandler

	s := &Server{
		echo:   e,
		config: cfg,
		logger: logger,
	}

	for _, opt := range opts {
		opt(s)
	}

	// Middleware order: Recovery -> BodyLimit -> Security -> CORS -> RequestID -> Metrics -> Logger -> Scope -> Handler
	e.Use(middleware.Recovery(logger))
	e.Use(echomw.BodyLimit(10 * 1024 * 1024)) // 10 MB
	e.Use(echomw.SecureWithConfig(echomw.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",
	}))
	if len(cfg.Server.AllowedOrigins) > 0 {
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			AllowOrigins: cfg.Server.AllowedOrigins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
			AllowHeaders: []string{"Authorization", "Content-Type", "X-Request-ID"},
		}))
	}
	e.Use(middleware.RequestID())
	e.Use(middleware.Metrics())
	e.Use(middleware.Logger(logger))
	e.Use(middleware.Scope())

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() { //nolint:gocognit,gocyclo,funlen // route registration is inherently complex
	healthH := handler.NewHealthHandler(s.pinger)

	s.echo.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})
	s.echo.GET("/healthz", healthH.Health)
	metricsHandler := echo.WrapHandler(promhttp.Handler())
	if s.config.Server.MetricsToken != "" {
		s.echo.GET("/metrics", metricsHandler, metricsTokenAuth(s.config.Server.MetricsToken))
	} else {
		s.echo.GET("/metrics", metricsHandler)
	}

	// Open-tracking pixel — public, no auth.
	if s.trackingHandler != nil {
		s.echo.GET("/t/o/:tracking_id", s.trackingHandler.HandleOpen)
	}

	// Video thumbnail composite image — public, no auth.
	if s.mediaHandler != nil {
		s.echo.GET("/public/video-thumbnail", s.mediaHandler.HandleVideoThumbnail)
	}

	// Data-plane API group.
	api := s.echo.Group("/api/v1")

	// POST /api/v1/send — API Key auth (HT-22).
	if s.sendHandler != nil {
		api.POST("/send", s.sendHandler.Send, middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper))
		api.POST("/send/batch", s.sendHandler.SendBatch, middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper))
	}

	// Data-plane email query endpoints — API Key auth.
	if s.dataPlaneEmailHandler != nil {
		api.GET("/emails", s.dataPlaneEmailHandler.List, middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper))
		api.GET("/emails/export", s.dataPlaneEmailHandler.Export, middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper))
		api.GET("/emails/:tracking_id", s.dataPlaneEmailHandler.GetByTrackingID, middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper))
		api.GET("/emails/:tracking_id/events", s.dataPlaneEmailHandler.GetEvents, middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper))
	}

	// SES webhook ingestion — NO AUTH, uses SNS signature verification (HT-23).
	if s.sesWebhookHandler != nil {
		api.POST("/webhooks/ses/inbound", s.sesWebhookHandler.HandleInbound)
	}

	// Onboarding — public status, OIDC setup (HT-25).
	if s.onboardingHandler != nil {
		api.GET("/onboarding/status", s.onboardingHandler.Status)
		api.POST("/onboarding/setup", s.onboardingHandler.Setup)
	}

	// Current authenticated member profile (OIDC only).
	if s.memberHandler != nil {
		api.GET("/members/me", s.memberHandler.Me, middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper), middleware.OIDCOnly())
	}

	// Management API (OIDC only) — only registered when handlers are provided.
	if s.tenantHandler != nil {
		mgmt := s.echo.Group("/api/v1/manage")
		mgmt.Use(middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper))
		mgmt.Use(middleware.OIDCOnly())

		// Tenants (superadmin).
		mgmt.POST("/tenants", s.tenantHandler.Create, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants", s.tenantHandler.List, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants/:tenant_code", s.tenantHandler.GetByCode, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.PUT("/tenants/:tenant_code", s.tenantHandler.Update, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.DELETE("/tenants/:tenant_code", s.tenantHandler.SoftDelete, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))

		// Workspaces (tenant_admin+).
		if s.workspaceHandler != nil { //nolint:dupl // repeated route group pattern
			mgmt.POST("/tenants/:tenant_code/workspaces", s.workspaceHandler.Create, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			mgmt.GET("/tenants/:tenant_code/workspaces", s.workspaceHandler.List, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			mgmt.GET("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
			mgmt.PUT("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			mgmt.DELETE("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.SoftDelete, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		}
		if s.workspacePolicyHandler != nil {
			mgmt.GET("/tenants/:tenant_code/workspaces/:workspace_code/policies", s.workspacePolicyHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
			mgmt.PUT("/tenants/:tenant_code/workspaces/:workspace_code/policies", s.workspacePolicyHandler.Update, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		}

		// Tenant dashboard stats.
		if s.dashboardHandler != nil {
			mgmt.GET("/tenants/:tenant_code/dashboard-stats", s.dashboardHandler.StatsTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		}

		// Members (superadmin).
		if s.memberHandler != nil {
			mgmt.GET("/members", s.memberHandler.List, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.POST("/members", s.memberHandler.Create, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.GET("/members/:member_id", s.memberHandler.Get, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.POST("/members/:member_id/roles", s.memberHandler.AddRole, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.DELETE("/members/:member_id/roles/:role_id", s.memberHandler.RemoveRole, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))

			tenantMembers := mgmt.Group("/tenants/:tenant_code")
			tenantMembers.GET("/members", s.memberHandler.ListTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			tenantMembers.POST("/members", s.memberHandler.Create, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			tenantMembers.GET("/members/:member_id", s.memberHandler.GetTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			tenantMembers.POST("/members/:member_id/roles", s.memberHandler.AddRoleTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			tenantMembers.DELETE("/members/:member_id/roles/:role_id", s.memberHandler.RemoveRoleTenant, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		}

		// Config (superadmin).
		if s.configHandler != nil {
			mgmt.GET("/config", s.configHandler.Get, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.PUT("/config", s.configHandler.Update, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		}

		// Workspace-scoped resources.
		{
			ws := mgmt.Group("/tenants/:tenant_code/workspaces/:workspace_code")

			if s.injectorHandler != nil { //nolint:dupl // repeated route group pattern
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
			if s.identityHandler != nil { //nolint:dupl // repeated route group pattern
				ws.GET("/adapters/:id/identities", s.identityHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.POST("/adapters/:id/identities", s.identityHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.POST("/adapters/:id/identities/sync", s.identityHandler.Sync, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.DELETE("/adapters/:id/identities/:identity_id", s.identityHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.POST("/adapters/:id/identities/:identity_id/set-default", s.identityHandler.SetDefault, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.GET("/adapters/:id/identities/:identity_id/workspace-access", s.identityHandler.GetWorkspaceAccess, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.PUT("/adapters/:id/identities/:identity_id/workspace-access", s.identityHandler.UpdateWorkspaceAccess, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			}
			// Template types (HT-21).
			if s.templateTypeHandler != nil { //nolint:dupl // repeated route group pattern
				ws.POST("/template-types", s.templateTypeHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.GET("/template-types", s.templateTypeHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.GET("/template-types/:slug", s.templateTypeHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.PUT("/template-types/:slug", s.templateTypeHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.DELETE("/template-types/:slug", s.templateTypeHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			}

			// Templates + versions + locales (HT-21).
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

			// API keys (HT-27).
			if s.apiKeyHandler != nil {
				ws.POST("/api-keys", s.apiKeyHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.GET("/api-keys", s.apiKeyHandler.List, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.DELETE("/api-keys/:id", s.apiKeyHandler.Revoke, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			}

			// Webhooks CRUD (HT-24).
			if s.webhookHandler != nil { //nolint:dupl // repeated route group pattern
				ws.POST("/webhooks", s.webhookHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.GET("/webhooks", s.webhookHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.GET("/webhooks/:id", s.webhookHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.PUT("/webhooks/:id", s.webhookHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.DELETE("/webhooks/:id", s.webhookHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.POST("/webhooks/:id/test", s.webhookHandler.Test, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			}

			// Emails query (HT-22).
			if s.emailHandler != nil {
				ws.GET("/emails", s.emailHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.GET("/emails/:tracking_id", s.emailHandler.GetByTrackingID, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.GET("/emails/:tracking_id/events", s.emailHandler.GetEvents, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
			}

			if s.memberHandler != nil { //nolint:dupl // repeated route group pattern
				ws.GET("/members", s.memberHandler.ListWorkspace, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.POST("/members", s.memberHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.GET("/members/:member_id", s.memberHandler.GetWorkspace, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.POST("/members/:member_id/roles", s.memberHandler.AddRoleWorkspace, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.DELETE("/members/:member_id/roles/:role_id", s.memberHandler.RemoveRoleWorkspace, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			}

			// Suppression list (HT-22).
			if s.suppressionHandler != nil {
				ws.POST("/suppression", s.suppressionHandler.Add, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
				ws.GET("/suppression/:email", s.suppressionHandler.Check, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
				ws.DELETE("/suppression/:email", s.suppressionHandler.Remove, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			}

			// Audit log (HT-22).
			if s.auditHandler != nil {
				ws.GET("/audit-log", s.auditHandler.Query, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
			}

			// Dashboard stats.
			if s.dashboardHandler != nil {
				ws.GET("/dashboard-stats", s.dashboardHandler.Stats, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
			}
		}

		// Global resources (superadmin only).
		{
			global := mgmt.Group("/global", middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))

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
			if s.identityHandler != nil { //nolint:dupl // repeated route group pattern
				global.GET("/adapters/:id/identities", s.identityHandler.ListGlobal)
				global.POST("/adapters/:id/identities", s.identityHandler.CreateGlobal)
				global.POST("/adapters/:id/identities/sync", s.identityHandler.SyncGlobal)
				global.DELETE("/adapters/:id/identities/:identity_id", s.identityHandler.DeleteGlobal)
				global.POST("/adapters/:id/identities/:identity_id/set-default", s.identityHandler.SetDefaultGlobal)
			}
			// Global template types (HT-21).
			if s.templateTypeHandler != nil {
				global.POST("/template-types", s.templateTypeHandler.CreateGlobal)
				global.GET("/template-types", s.templateTypeHandler.ListGlobal)
				global.GET("/template-types/:slug", s.templateTypeHandler.GetGlobal)
				global.PUT("/template-types/:slug", s.templateTypeHandler.UpdateGlobal)
				global.DELETE("/template-types/:slug", s.templateTypeHandler.DeleteGlobal)
			}

			// Global templates (HT-21).
			if s.templateHandler != nil {
				global.GET("/template-types/:slug/templates", s.templateHandler.ListByTemplateTypeGlobal)
				global.POST("/templates", s.templateHandler.CreateTemplateGlobal)
				// Version CRUD — handlers operate on template_id directly, no workspace needed.
				global.GET("/templates/:template_id/versions", s.templateHandler.ListVersions)
				global.GET("/templates/:template_id/versions/:version_id", s.templateHandler.GetVersion)
				global.POST("/templates/:template_id/versions", s.templateHandler.CreateVersion)
				global.PUT("/templates/:template_id/versions/:version_id", s.templateHandler.UpdateVersion)
				global.POST("/templates/:template_id/versions/:version_id/clone", s.templateHandler.CloneVersionGlobal)
				global.POST("/templates/:template_id/versions/:version_id/publish", s.templateHandler.PublishVersion)
				// Locale CRUD.
				global.GET("/templates/:template_id/versions/:version_id/locales", s.templateHandler.ListLocales)
				global.POST("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.SetLocale)
				global.PUT("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.UpdateLocale)
				global.GET("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.GetLocale)
				global.DELETE("/templates/:template_id/versions/:version_id/locales/:locale", s.templateHandler.DeleteLocale)
				// Preview.
				global.POST("/templates/:template_id/preview-mjml", s.templateHandler.PreviewMJML)
				global.POST("/templates/:template_id/test-send", s.templateHandler.TestSend)
				global.POST("/templates/:template_id/disable", s.templateHandler.DisableTemplateGlobal)
				global.POST("/templates/:template_id/enable", s.templateHandler.EnableTemplateGlobal)
				global.DELETE("/templates/:template_id", s.templateHandler.DeleteTemplate)
				global.DELETE("/templates/:template_id/versions/:version_id", s.templateHandler.DeleteVersion)
			}

			// Global audit log (HT-22).
			if s.auditHandler != nil {
				global.GET("/audit-log", s.auditHandler.QueryGlobal)
			}

			// Global dashboard stats.
			if s.dashboardHandler != nil {
				global.GET("/dashboard-stats", s.dashboardHandler.StatsGlobal)
			}
		}
	}
}

// Start runs the HTTP server with graceful shutdown on context cancellation.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	s.logger.Info("starting server", slog.String("address", addr))

	sc := echo.StartConfig{
		Address:         addr,
		GracefulTimeout: s.config.Server.ShutdownTimeout,
	}
	return sc.Start(ctx, s.echo)
}

// Echo returns the underlying echo instance (for testing).
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// metricsTokenAuth returns middleware that requires a Bearer token matching
// the configured metrics token. Used to protect the /metrics endpoint.
// Uses constant-time comparison to prevent timing side-channel attacks.
func metricsTokenAuth(token string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			provided := strings.TrimPrefix(auth, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			return next(c)
		}
	}
}

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
	"github.com/rendis/senda/config"
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

	// workspaceExistenceStore is used by the external integration middleware to
	// construct a per-request workspace filter for resolver extensions.
	workspaceExistenceStore port.WorkspaceExistenceStore

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
	templateTypeHandler        *handler.TemplateTypeHandler
	templateHandler            *handler.TemplateHandler
	templateScreenshotHandler  *handler.TemplateScreenshotHandler

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

	// External integration handler.
	externalIntegrationHandler *handler.ExternalIntegrationHandler

	// HT-27 handler (API Keys).
	apiKeyHandler *handler.APIKeyHandler

	// Dashboard handler.
	dashboardHandler *handler.DashboardHandler

	// Open-tracking handler (public, no auth).
	trackingHandler *handler.TrackingHandler

	// Media handler (public, no auth).
	mediaHandler *handler.MediaHandler

	// Unsubscribe handler (public, no auth).
	unsubscribeHandler *handler.UnsubscribeHandler
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

// WithWorkspaceExistenceStore sets the WorkspaceExistenceStore used by the
// external integration middleware to build a per-request workspace filter.
func WithWorkspaceExistenceStore(ws port.WorkspaceExistenceStore) ServerOption {
	return func(s *Server) {
		s.workspaceExistenceStore = ws
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

// WithTemplateScreenshotHandler sets the TemplateScreenshotHandler.
func WithTemplateScreenshotHandler(h *handler.TemplateScreenshotHandler) ServerOption {
	return func(s *Server) {
		s.templateScreenshotHandler = h
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

// WithExternalIntegrationHandler sets the handler for the external integration
// bootstrap route.
func WithExternalIntegrationHandler(h *handler.ExternalIntegrationHandler) ServerOption {
	return func(s *Server) {
		s.externalIntegrationHandler = h
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

// WithUnsubscribeHandler sets the UnsubscribeHandler for public unsubscribe routes.
func WithUnsubscribeHandler(h *handler.UnsubscribeHandler) ServerOption {
	return func(s *Server) {
		s.unsubscribeHandler = h
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
	if s.externalIntegrationHandler != nil {
		e.Use(middleware.ExternalIntegrationCORS(s.externalIntegrationHandler))
	}
	if len(cfg.Server.AllowedOrigins) > 0 {
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			Skipper: func(c *echo.Context) bool {
				return strings.HasPrefix(c.Request().URL.Path, "/api/v1/external/")
			},
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

func (s *Server) registerRoutes() {
	s.registerPublicRoutes()

	api := s.echo.Group("/api/v1")
	s.registerDataPlaneRoutes(api)
	s.registerProviderRoutes(api)
	s.registerOnboardingRoutes(api)
	s.registerExternalRoutes()
	s.registerManagementRoutes()
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

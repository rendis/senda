package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/senda-app/senda/config"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
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

	// Store dependencies for RBAC middleware.
	tenantStore port.TenantStore
	wsStore     port.WorkspaceStore
	configStore port.GlobalConfigStore

	// Handlers.
	tenantHandler    *handler.TenantHandler
	workspaceHandler *handler.WorkspaceHandler
	memberHandler    *handler.MemberHandler
	configHandler    *handler.ConfigHandler
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
func WithAuthDeps(apiKeyStore port.APIKeyStore, memberStore port.MemberStore, oidcVerifier port.OIDCVerifier) ServerOption {
	return func(s *Server) {
		s.apiKeyStore = apiKeyStore
		s.memberStore = memberStore
		s.oidcVerifier = oidcVerifier
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

	// Middleware order: Recovery -> RequestID -> Metrics -> Logger -> Scope -> Handler
	e.Use(middleware.Recovery(logger))
	e.Use(middleware.RequestID())
	e.Use(middleware.Metrics())
	e.Use(middleware.Logger(logger))
	e.Use(middleware.Scope())

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	healthH := handler.NewHealthHandler(s.pinger)

	s.echo.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})
	s.echo.GET("/healthz", healthH.Health)
	s.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// Data-plane API group (future: send emails, query events, etc.).
	_ = s.echo.Group("/api/v1")

	// Management API (OIDC only) — only registered when handlers are provided.
	if s.tenantHandler != nil {
		mgmt := s.echo.Group("/api/v1/manage")
		mgmt.Use(middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier))
		mgmt.Use(middleware.OIDCOnly())

		// Tenants (superadmin).
		mgmt.POST("/tenants", s.tenantHandler.Create, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants", s.tenantHandler.List, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		mgmt.GET("/tenants/:tenant_code", s.tenantHandler.GetByCode, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.PUT("/tenants/:tenant_code", s.tenantHandler.Update, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		mgmt.DELETE("/tenants/:tenant_code", s.tenantHandler.SoftDelete, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))

		// Workspaces (tenant_admin+).
		if s.workspaceHandler != nil {
			mgmt.POST("/tenants/:tenant_code/workspaces", s.workspaceHandler.Create, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			mgmt.GET("/tenants/:tenant_code/workspaces", s.workspaceHandler.List, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
			mgmt.GET("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer, s.tenantStore, s.wsStore))
			mgmt.PUT("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin, s.tenantStore, s.wsStore))
			mgmt.DELETE("/tenants/:tenant_code/workspaces/:workspace_code", s.workspaceHandler.SoftDelete, middleware.RequireRole(domain.RoleTenantAdmin, s.tenantStore, s.wsStore))
		}

		// Members (superadmin).
		if s.memberHandler != nil {
			mgmt.POST("/members", s.memberHandler.Create, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.GET("/members/:member_id", s.memberHandler.Get, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.POST("/members/:member_id/roles", s.memberHandler.AddRole, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.DELETE("/members/:member_id/roles/:role_id", s.memberHandler.RemoveRole, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
		}

		// Config (superadmin).
		if s.configHandler != nil {
			mgmt.GET("/config", s.configHandler.Get, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
			mgmt.PUT("/config", s.configHandler.Update, middleware.RequireRole(domain.RoleSuperadmin, s.tenantStore, s.wsStore))
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

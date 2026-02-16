package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/config"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
)

// Server wraps the Echo instance with application configuration and logger.
type Server struct {
	echo   *echo.Echo
	config *config.Config
	logger *slog.Logger
}

// NewServer creates a configured Echo server with middleware and routes.
func NewServer(cfg *config.Config, logger *slog.Logger) *Server {
	e := echo.New()

	e.HTTPErrorHandler = response.HTTPErrorHandler

	// Middleware order: Recovery -> RequestID -> Logger -> Scope -> Handler
	e.Use(middleware.Recovery(logger))
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger(logger))
	e.Use(middleware.Scope())

	s := &Server{
		echo:   e,
		config: cfg,
		logger: logger,
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	s.echo.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// Placeholder route groups for future API handlers.
	_ = s.echo.Group("/api/v1")
	_ = s.echo.Group("/api/v1/manage")
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

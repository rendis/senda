package http

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rendis/senda/internal/http/handler"
)

func (s *Server) registerPublicRoutes() {
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

	if s.trackingHandler != nil {
		s.echo.GET("/t/o/:tracking_id", s.trackingHandler.HandleOpen)
	}

	if s.mediaHandler != nil {
		s.echo.GET("/public/video-thumbnail", s.mediaHandler.HandleVideoThumbnail)
	}
}

func (s *Server) registerOnboardingRoutes(api *echo.Group) {
	if s.onboardingHandler == nil {
		return
	}

	api.GET("/onboarding/status", s.onboardingHandler.Status)
	api.POST("/onboarding/setup", s.onboardingHandler.Setup)
}

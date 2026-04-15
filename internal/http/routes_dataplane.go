package http

import (
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/middleware"
)

func (s *Server) registerDataPlaneRoutes(api *echo.Group) {
	auth := middleware.Auth(s.apiKeyStore, s.memberStore, s.oidcVerifier, s.apiKeyPepper)

	if s.sendHandler != nil {
		api.POST("/send", s.sendHandler.Send, auth)
		api.POST("/send/batch", s.sendHandler.SendBatch, auth)
	}

	if s.dataPlaneEmailHandler != nil {
		api.GET("/emails", s.dataPlaneEmailHandler.List, auth)
		api.GET("/emails/export", s.dataPlaneEmailHandler.Export, auth)
		api.GET("/emails/:tracking_id", s.dataPlaneEmailHandler.GetByTrackingID, auth)
		api.GET("/emails/:tracking_id/events", s.dataPlaneEmailHandler.GetEvents, auth)
	}
}

package http

import "github.com/labstack/echo/v5"

func (s *Server) registerProviderRoutes(api *echo.Group) {
	if s.sesWebhookHandler == nil {
		return
	}

	api.POST("/webhooks/ses/inbound", s.sesWebhookHandler.HandleInbound)
}

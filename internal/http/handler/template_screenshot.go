package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/pkg/apperr"
)

// ScreenshotService is the contract the handler depends on.
type ScreenshotService interface {
	Capture(ctx context.Context, in service.ScreenshotInput) ([]byte, error)
}

// TemplateScreenshotHandler exposes GET .../templates/:template_id/screenshot.
type TemplateScreenshotHandler struct {
	svc ScreenshotService
}

// NewTemplateScreenshotHandler wires the handler.
func NewTemplateScreenshotHandler(svc ScreenshotService) *TemplateScreenshotHandler {
	return &TemplateScreenshotHandler{svc: svc}
}

// Capture handles GET .../templates/:template_id/screenshot.
func (h *TemplateScreenshotHandler) Capture(c *echo.Context) error {
	templateID, err := uuid.Parse(c.Param("template_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "INVALID_TEMPLATE_ID", "invalid template id")
	}

	q := request.TemplateScreenshotQuery{}
	if err := c.Bind(&q); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid query parameters")
	}
	if q.Viewport == "" {
		q.Viewport = "desktop"
	}

	in := service.ScreenshotInput{
		TemplateID: templateID,
		Viewport:   q.Viewport,
		Locale:     q.Locale,
	}
	if q.VersionID != "" {
		vid, err := uuid.Parse(q.VersionID)
		if err != nil {
			return response.WriteError(c, http.StatusBadRequest, "INVALID_VERSION_ID", "invalid version_id")
		}
		in.VersionID = &vid
	}

	png, err := h.svc.Capture(c.Request().Context(), in)
	if err != nil {
		return mapScreenshotError(c, err)
	}
	return c.Blob(http.StatusOK, "image/png", png)
}

func mapScreenshotError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrScreenshotDisabled):
		return response.WriteError(c, http.StatusServiceUnavailable, "SCREENSHOT_DISABLED", "screenshot feature disabled")
	case errors.Is(err, service.ErrInvalidViewport):
		return response.WriteError(c, http.StatusBadRequest, "INVALID_VIEWPORT", "viewport must be desktop or mobile")
	case errors.Is(err, service.ErrScreenshotBusy):
		return response.WriteError(c, http.StatusServiceUnavailable, "SCREENSHOT_BUSY", "all screenshot slots in use")
	case errors.Is(err, service.ErrScreenshotTimeout):
		return response.WriteError(c, http.StatusGatewayTimeout, "SCREENSHOT_TIMEOUT", "screenshot timed out")
	case errors.Is(err, domain.ErrTemplateNotFound):
		return response.WriteError(c, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "template not found")
	case errors.Is(err, domain.ErrTemplateDisabled):
		return response.WriteError(c, http.StatusConflict, "CONFLICT", "template is disabled")
	case errors.Is(err, domain.ErrNoPublishedVersion):
		return response.WriteError(c, http.StatusNotFound, "NO_PUBLISHED_VERSION", "no published version")
	case errors.Is(err, service.ErrScreenshotInternal):
		return response.WriteError(c, http.StatusInternalServerError, "SCREENSHOT_INTERNAL", "screenshot capture failed")
	}

	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case http.StatusNotFound:
			return response.WriteError(c, http.StatusNotFound, "TEMPLATE_NOT_FOUND", appErr.Message)
		case http.StatusConflict:
			return response.WriteError(c, http.StatusConflict, "CONFLICT", appErr.Message)
		}
	}
	return response.WriteError(c, http.StatusInternalServerError, "INTERNAL", "internal error")
}

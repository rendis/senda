package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/internal/tracking"
)

// TrackingHandler serves the open-tracking pixel endpoint.
type TrackingHandler struct {
	lifecycleCtx   context.Context
	emailStore     trackingEmailLookup
	eventProcessor *service.EventProcessor
	logger         *slog.Logger
}

// trackingEmailLookup is the minimal interface needed for tracking lookups.
type trackingEmailLookup interface {
	GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error)
}

// NewTrackingHandler creates a new TrackingHandler.
// The lifecycleCtx should be the server's lifecycle context so background
// goroutines are cancelled on graceful shutdown rather than leaked.
func NewTrackingHandler(lifecycleCtx context.Context, es trackingEmailLookup, ep *service.EventProcessor, logger *slog.Logger) *TrackingHandler {
	return &TrackingHandler{lifecycleCtx: lifecycleCtx, emailStore: es, eventProcessor: ep, logger: logger}
}

// HandleOpen serves GET /t/o/:tracking_id — returns a 1x1 transparent GIF
// and records an "opened" event asynchronously.
func (h *TrackingHandler) HandleOpen(c *echo.Context) error {
	trackingID := c.Param("tracking_id")
	if trackingID == "" {
		return c.Blob(http.StatusOK, "image/gif", tracking.TransparentGIF)
	}

	// Fire-and-forget: record open event in a goroutine so the pixel
	// response is never delayed by database calls.
	// Uses lifecycleCtx so the goroutine is cancelled on server shutdown.
	go func() {
		ctx, cancel := context.WithTimeout(h.lifecycleCtx, 5*time.Second)
		defer cancel()

		email, err := h.emailStore.GetByTrackingID(ctx, trackingID)
		if err != nil {
			h.logger.Warn("tracking: email not found", "tracking_id", trackingID, "error", err)
			return
		}

		event := &domain.ProviderEvent{
			Type:              domain.EventOpened,
			ProviderMessageID: "", // not applicable for pixel opens
			Timestamp:         time.Now().UTC(),
		}

		if err := h.eventProcessor.ProcessDirect(ctx, email, event); err != nil {
			h.logger.Error("tracking: failed to process open event",
				"tracking_id", trackingID,
				"email_id", email.ID,
				"error", err,
			)
		}
	}()

	// Always return the pixel immediately, regardless of event processing.
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Response().Header().Set("Pragma", "no-cache")
	c.Response().Header().Set("Expires", "0")
	return c.Blob(http.StatusOK, "image/gif", tracking.TransparentGIF)
}

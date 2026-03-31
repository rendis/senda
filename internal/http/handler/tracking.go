package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/internal/tracking"
)

// dedupTTL is the window during which a repeated tracking_id open is suppressed.
const dedupTTL = 30 * time.Second

// TrackingHandler serves the open-tracking pixel endpoint.
type TrackingHandler struct {
	lifecycleCtx   context.Context
	emailStore     trackingEmailLookup
	eventProcessor *service.EventProcessor
	logger         *slog.Logger

	// recentOpens is an in-memory dedup cache: tracking_id → time of last processed open.
	// Entries older than dedupTTL are evicted when the map exceeds evictionThreshold.
	recentMu      sync.Mutex
	recentOpens   map[string]time.Time
	lastEviction  time.Time

	// wg tracks in-flight fire-and-forget goroutines so Drain() can wait for them.
	wg sync.WaitGroup
}

// trackingEmailLookup is the minimal interface needed for tracking lookups.
type trackingEmailLookup interface {
	GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error)
}

// NewTrackingHandler creates a new TrackingHandler.
// The lifecycleCtx should be the server's lifecycle context so background
// goroutines are cancelled on graceful shutdown rather than leaked.
func NewTrackingHandler(lifecycleCtx context.Context, es trackingEmailLookup, ep *service.EventProcessor, logger *slog.Logger) *TrackingHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TrackingHandler{
		lifecycleCtx: lifecycleCtx,
		emailStore:   es,
		eventProcessor: ep,
		logger:       logger,
		recentOpens:  make(map[string]time.Time),
	}
}

// HandleOpen serves GET /t/o/:tracking_id — returns a 1x1 transparent GIF
// and records an "opened" event asynchronously.
//
// Dedup: if the same tracking_id was already processed within the last 30 seconds,
// the goroutine is skipped to prevent goroutine floods from a single tracking_id.
// The pixel is always returned regardless.
func (h *TrackingHandler) HandleOpen(c *echo.Context) error {
	trackingID := c.Param("tracking_id")
	if trackingID == "" {
		return c.Blob(http.StatusOK, "image/gif", tracking.TransparentGIF)
	}

	// Dedup check: skip goroutine if this tracking_id was processed recently.
	if h.isDuplicate(trackingID) {
		return returnPixel(c)
	}

	// Fire-and-forget: record open event in a goroutine so the pixel
	// response is never delayed by database calls.
	// Uses lifecycleCtx so the goroutine is cancelled on server shutdown.
	// wg.Add/Done lets Drain() wait for all in-flight goroutines on shutdown.
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
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
	return returnPixel(c)
}

// returnPixel writes a 1×1 transparent GIF with no-cache headers.
func returnPixel(c *echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Response().Header().Set("Pragma", "no-cache")
	c.Response().Header().Set("Expires", "0")
	return c.Blob(http.StatusOK, "image/gif", tracking.TransparentGIF)
}

// Drain blocks until all in-flight open-tracking goroutines have finished.
// Call this during graceful shutdown after stopping new request acceptance.
func (h *TrackingHandler) Drain() {
	h.wg.Wait()
}

// isDuplicate returns true if trackingID was seen within the last dedupTTL window.
// Eviction is amortized: only runs when enough time has passed since the last sweep.
func (h *TrackingHandler) isDuplicate(trackingID string) bool {
	now := time.Now()

	h.recentMu.Lock()
	defer h.recentMu.Unlock()

	// Amortized eviction: sweep at most once per dedupTTL/2 to keep the hot path O(1).
	if now.Sub(h.lastEviction) >= dedupTTL/2 {
		for id, ts := range h.recentOpens {
			if now.Sub(ts) >= dedupTTL {
				delete(h.recentOpens, id)
			}
		}
		h.lastEviction = now
	}

	if ts, found := h.recentOpens[trackingID]; found && now.Sub(ts) < dedupTTL {
		return true
	}

	h.recentOpens[trackingID] = now
	return false
}

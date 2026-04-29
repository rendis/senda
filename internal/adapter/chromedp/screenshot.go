package chromedp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	cdp "github.com/chromedp/chromedp"

	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/port"
)

// Capturer implements port.ScreenshotCapture using a long-running Chromium.
type Capturer struct {
	pool *Pool
	cfg  config.ScreenshotConfig
}

// NewCapturer creates a Capturer backed by the given Pool.
func NewCapturer(pool *Pool, cfg config.ScreenshotConfig) *Capturer {
	return &Capturer{pool: pool, cfg: cfg}
}

// Capture renders html under the given viewport and returns a PNG.
// The captured height is capped at maxHeightPx.
func (c *Capturer) Capture(ctx context.Context, html string, vp port.Viewport, maxHeightPx int) ([]byte, error) {
	if html == "" {
		return nil, errors.New("chromedp: empty html")
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	release, err := c.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("chromedp: acquire: %w", err)
	}
	defer release()

	allocCtx, err := c.pool.AllocatorContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("chromedp: allocator: %w", err)
	}

	browserCtx, browserCancel := cdp.NewContext(allocCtx)
	defer browserCancel()

	// Encode the HTML as a base64 data URL to avoid filesystem and network dependencies.
	// base64 is robust against all characters (spaces, UTF-8, special chars, emojis).
	dataURL := "data:text/html;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(html))

	var (
		scrollHeight int64
		png          []byte
	)
	cappedHeight := int64(maxHeightPx)

	err = cdp.Run(browserCtx,
		emulation.SetDeviceMetricsOverride(int64(vp.WidthPx), int64(maxHeightPx), vp.DeviceScale, vp.MobileEmul),
		emulation.SetTouchEmulationEnabled(vp.MobileEmul),
		cdp.Navigate(dataURL),
		cdp.WaitReady("body"),
		cdp.Evaluate(`document.documentElement.scrollHeight`, &scrollHeight),
		cdp.ActionFunc(func(ctx context.Context) error {
			h := min(scrollHeight, cappedHeight)
			if h <= 0 {
				h = 1
			}
			return emulation.SetDeviceMetricsOverride(int64(vp.WidthPx), h, vp.DeviceScale, vp.MobileEmul).Do(ctx)
		}),
		cdp.ActionFunc(func(ctx context.Context) error {
			data, err := page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatPng).
				WithCaptureBeyondViewport(true).
				WithFromSurface(true).
				Do(ctx)
			if err != nil {
				return err
			}
			png = data
			return nil
		}),
	)
	if err != nil {
		if isFatal(err) {
			c.pool.Restart()
		}
		return nil, fmt.Errorf("chromedp: capture: %w", err)
	}
	return png, nil
}

// isFatal reports whether err indicates the browser process is gone.
func isFatal(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "websocket: close") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "browser disconnected")
}

// Compile-time interface check.
var _ port.ScreenshotCapture = (*Capturer)(nil)

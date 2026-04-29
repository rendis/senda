package port

import "context"

// Viewport describes how Chromium should emulate the device for a capture.
type Viewport struct {
	Name        string  // "desktop" | "mobile"
	WidthPx     int
	DeviceScale float64
	MobileEmul  bool // touch + mobile UA-CH
}

// ScreenshotCapture renders an HTML document to a full-page PNG.
//
// Implementations MUST honor ctx cancellation and ctx deadlines, and MUST
// cap the captured height at maxHeightPx. The returned bytes are a complete
// PNG file.
type ScreenshotCapture interface {
	Capture(ctx context.Context, html string, vp Viewport, maxHeightPx int) ([]byte, error)
}

//go:build integration

package chromedp_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rendis/senda/config"
	chromedpadapter "github.com/rendis/senda/internal/adapter/chromedp"
	"github.com/rendis/senda/internal/port"
)

func TestCapture_Smoke(t *testing.T) {
	chromiumPath := os.Getenv("SENDA_TEST_CHROMIUM_PATH")
	if chromiumPath == "" {
		t.Skip("set SENDA_TEST_CHROMIUM_PATH to run integration test")
	}
	cfg := config.ScreenshotConfig{
		Enabled:        true,
		ChromiumPath:   chromiumPath,
		Timeout:        15 * time.Second,
		MaxHeightPx:    2000,
		MaxConcurrent:  1,
		DesktopWidthPx: 1280,
		MobileWidthPx:  390,
	}
	pool := chromedpadapter.New(cfg, slog.Default())
	defer pool.Stop(context.Background())

	cap := chromedpadapter.NewCapturer(pool, cfg)

	html := `<!doctype html><html><body style="margin:0;background:#f00">
		<div style="height:300px;background:#0f0">desktop preview</div>
	</body></html>`

	png, err := cap.Capture(context.Background(), html, port.Viewport{
		Name: "desktop", WidthPx: 1280, DeviceScale: 1.0, MobileEmul: false,
	}, 2000)
	require.NoError(t, err)
	require.NotEmpty(t, png)
	require.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47}, png[:4], "PNG header")
}

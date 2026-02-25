package handler

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v5"
	sendaresponse "github.com/senda-app/senda/internal/http/response"
)

const (
	maxDownloadBytes  = 10 * 1024 * 1024 // 10 MB
	downloadTimeout   = 10 * time.Second
	cacheControlValue = "public, max-age=86400"
	maxCacheEntries   = 500
)

// MediaHandler serves public media utility endpoints.
type MediaHandler struct {
	logger    *slog.Logger
	cache     sync.Map // map[string][]byte — keyed by thumbnail URL, stores PNG bytes
	cacheSize atomic.Int64
	client    *http.Client
	skipSSRF  bool
}

// MediaHandlerOption configures optional MediaHandler settings.
type MediaHandlerOption func(*MediaHandler)

// WithSkipSSRF disables SSRF validation (for testing only).
func WithSkipSSRF() MediaHandlerOption {
	return func(h *MediaHandler) { h.skipSSRF = true }
}

// NewMediaHandler creates a new MediaHandler.
func NewMediaHandler(logger *slog.Logger, opts ...MediaHandlerOption) *MediaHandler {
	h := &MediaHandler{
		logger: logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	h.client = &http.Client{
		Timeout: downloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !h.skipSSRF {
				if err := checkSSRF(req.URL); err != nil {
					return err
				}
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	return h
}

// checkSSRF resolves the host of u and returns an error if any resolved address
// is a loopback, private, link-local, or otherwise disallowed address.
func checkSSRF(u *url.URL) error {
	host := u.Hostname()
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("could not resolve host %q: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			return fmt.Errorf("could not parse resolved address %q", addr)
		}
		if isDisallowedIP(ip) {
			return fmt.Errorf("url resolves to a disallowed address")
		}
	}
	return nil
}

// isDisallowedIP returns true if ip is loopback, private, link-local,
// unspecified, or otherwise disallowed for outbound proxy requests.
func isDisallowedIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	if ip.IsUnspecified() {
		return true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsMulticast() {
		return true
	}

	// Private IPv4 ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16.
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",  // Carrier-grade NAT
		"192.0.0.0/24",   // IETF Protocol Assignments
		"198.18.0.0/15",  // Benchmarking
		"198.51.100.0/24", // Documentation
		"203.0.113.0/24", // Documentation
		"240.0.0.0/4",    // Reserved
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// HandleVideoThumbnail serves GET /public/video-thumbnail?url=<encoded-thumbnail-url>.
// It downloads the thumbnail, composites a play-button overlay, and returns a PNG.
// The result is cached in memory keyed by the raw thumbnail URL.
func (h *MediaHandler) HandleVideoThumbnail(c *echo.Context) error {
	rawURL := c.QueryParam("url")
	if rawURL == "" {
		return sendaresponse.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "url query parameter is required")
	}

	// Validate scheme — only http and https are allowed.
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return sendaresponse.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "url must use http or https scheme")
	}

	// SSRF check — reject requests to private/internal addresses.
	if !h.skipSSRF {
		if err := checkSSRF(parsed); err != nil {
			return sendaresponse.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "url resolves to a disallowed address")
		}
	}

	// Serve from cache if available.
	if cached, ok := h.cache.Load(rawURL); ok {
		pngBytes := cached.([]byte)
		c.Response().Header().Set("Cache-Control", cacheControlValue)
		return c.Blob(http.StatusOK, "image/png", pngBytes)
	}

	// Download the thumbnail.
	pngBytes, err := h.buildComposite(rawURL)
	if err != nil {
		h.logger.Warn("media: failed to build video thumbnail composite",
			"url", rawURL,
			"error", err,
		)
		return sendaresponse.WriteError(c, http.StatusBadGateway, "BAD_GATEWAY", "could not retrieve thumbnail")
	}

	// Store in cache only if under the entry cap.
	if h.cacheSize.Load() < maxCacheEntries {
		h.cache.Store(rawURL, pngBytes)
		h.cacheSize.Add(1)
	}

	c.Response().Header().Set("Cache-Control", cacheControlValue)
	return c.Blob(http.StatusOK, "image/png", pngBytes)
}

// buildComposite downloads the image at rawURL, draws the play-button overlay,
// and returns the result encoded as PNG bytes.
func (h *MediaHandler) buildComposite(rawURL string) ([]byte, error) {
	resp, err := h.client.Get(rawURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download: upstream returned %d", resp.StatusCode)
	}

	// Guard against oversized images.
	limited := io.LimitReader(resp.Body, maxDownloadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(data) > maxDownloadBytes {
		return nil, fmt.Errorf("thumbnail exceeds 10 MB limit")
	}

	// Decode the image (JPEG and PNG are the common thumbnail formats).
	src, err := decodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Composite: draw thumbnail + play-button overlay onto an RGBA canvas.
	bounds := src.Bounds()
	canvas := image.NewRGBA(bounds)

	// Draw the thumbnail as background.
	draw.Draw(canvas, bounds, src, bounds.Min, draw.Src)

	// Draw the play-button overlay centered on the canvas.
	drawPlayButton(canvas, bounds)

	// Encode to PNG.
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeImage decodes raw bytes as JPEG or PNG, falling back to the standard
// image.Decode function for other registered formats.
func decodeImage(data []byte) (image.Image, error) {
	// Try JPEG first (most common thumbnail format from YouTube / Vimeo).
	if img, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	// Try PNG.
	if img, err := png.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	// Try any other format registered via init() (e.g. GIF via image/gif).
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("unsupported or corrupt image format")
	}
	_ = format
	return img, nil
}

// drawPlayButton renders a semi-transparent dark circle with a white right-pointing
// triangle (play icon) centred within it, over the given RGBA canvas.
// Images smaller than 40×40 pixels are skipped to avoid out-of-bounds drawing.
func drawPlayButton(canvas *image.RGBA, bounds image.Rectangle) {
	w := bounds.Dx()
	h := bounds.Dy()

	// Skip drawing on images that are too small to safely render the overlay.
	if w < 40 || h < 40 {
		return
	}

	cx := bounds.Min.X + w/2
	cy := bounds.Min.Y + h/2

	// Circle radius: ~25% of image height.
	radius := int(math.Round(float64(h) * 0.25))
	if radius < 10 {
		radius = 10
	}

	// Semi-transparent dark background circle.
	circleColor := color.RGBA{R: 0, G: 0, B: 0, A: 160} // ~63% opacity
	drawFilledCircle(canvas, cx, cy, radius, circleColor)

	// White play triangle centred inside the circle.
	// The triangle points right; its bounding box is ~50% of the circle diameter.
	triHalfH := int(math.Round(float64(radius) * 0.50))
	triWidth := int(math.Round(float64(radius) * 0.60))

	// Slight rightward nudge so the triangle looks visually centred.
	nudge := radius / 10

	// Triangle vertices (pointing right):
	//   apex  — rightmost point
	//   topL  — upper-left
	//   botL  — lower-left
	apex := image.Point{X: cx + triWidth/2 + nudge, Y: cy}
	topL := image.Point{X: cx - triWidth/2 + nudge, Y: cy - triHalfH}
	botL := image.Point{X: cx - triWidth/2 + nudge, Y: cy + triHalfH}

	drawFilledTriangle(canvas, apex, topL, botL, color.White)
}

// drawFilledCircle rasterises a filled circle using the midpoint algorithm.
func drawFilledCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	rr := color.RGBAModel.Convert(c).(color.RGBA)
	for y := -r; y <= r; y++ {
		// Horizontal span at this y offset.
		dx := int(math.Round(math.Sqrt(float64(r*r - y*y))))
		for x := -dx; x <= dx; x++ {
			px := cx + x
			py := cy + y
			img.SetRGBA(px, py, blend(img.RGBAAt(px, py), rr))
		}
	}
}

// drawFilledTriangle fills the triangle defined by three vertices using a
// scanline rasteriser.  All three points must lie within the image bounds.
func drawFilledTriangle(img *image.RGBA, p0, p1, p2 image.Point, c color.Color) {
	fill := color.RGBAModel.Convert(c).(color.RGBA)

	// Sort vertices by Y so p0 is topmost.
	pts := [3]image.Point{p0, p1, p2}
	sortByY(&pts)
	top, mid, bot := pts[0], pts[1], pts[2]

	totalH := bot.Y - top.Y
	if totalH == 0 {
		return
	}

	for scanY := top.Y; scanY <= bot.Y; scanY++ {
		// Left edge: always top→bot.
		t1 := float64(scanY-top.Y) / float64(totalH)
		x1 := lerp(float64(top.X), float64(bot.X), t1)

		// Right edge: top→mid for upper half, mid→bot for lower half.
		var x2 float64
		if scanY < mid.Y {
			segH := float64(mid.Y - top.Y)
			if segH == 0 {
				x2 = float64(mid.X)
			} else {
				t2 := float64(scanY-top.Y) / segH
				x2 = lerp(float64(top.X), float64(mid.X), t2)
			}
		} else {
			segH := float64(bot.Y - mid.Y)
			if segH == 0 {
				x2 = float64(mid.X)
			} else {
				t2 := float64(scanY-mid.Y) / segH
				x2 = lerp(float64(mid.X), float64(bot.X), t2)
			}
		}

		xLeft := int(math.Round(math.Min(x1, x2)))
		xRight := int(math.Round(math.Max(x1, x2)))
		for x := xLeft; x <= xRight; x++ {
			img.SetRGBA(x, scanY, blend(img.RGBAAt(x, scanY), fill))
		}
	}
}

// blend alpha-composites src over dst (src-over).
func blend(dst, src color.RGBA) color.RGBA {
	if src.A == 255 {
		return src
	}
	if src.A == 0 {
		return dst
	}
	srcA := float64(src.A) / 255.0
	dstA := float64(dst.A) / 255.0
	outA := srcA + dstA*(1-srcA)
	if outA == 0 {
		return color.RGBA{}
	}
	outR := (float64(src.R)*srcA + float64(dst.R)*dstA*(1-srcA)) / outA
	outG := (float64(src.G)*srcA + float64(dst.G)*dstA*(1-srcA)) / outA
	outB := (float64(src.B)*srcA + float64(dst.B)*dstA*(1-srcA)) / outA
	return color.RGBA{
		R: uint8(math.Round(outR)),
		G: uint8(math.Round(outG)),
		B: uint8(math.Round(outB)),
		A: uint8(math.Round(outA * 255)),
	}
}

// sortByY sorts the three points so pts[0].Y <= pts[1].Y <= pts[2].Y.
func sortByY(pts *[3]image.Point) {
	if pts[0].Y > pts[1].Y {
		pts[0], pts[1] = pts[1], pts[0]
	}
	if pts[1].Y > pts[2].Y {
		pts[1], pts[2] = pts[2], pts[1]
	}
	if pts[0].Y > pts[1].Y {
		pts[0], pts[1] = pts[1], pts[0]
	}
}

// lerp linearly interpolates between a and b by t ∈ [0,1].
func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

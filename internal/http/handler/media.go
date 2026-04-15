package handler

import (
	"bytes"
	"container/list"
	"context"
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
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	sendaresponse "github.com/rendis/senda/internal/http/response"
)

const (
	maxDownloadBytes             = 10 * 1024 * 1024 // 10 MB
	defaultDownloadTimeout       = 10 * time.Second
	defaultThumbnailCacheTTL     = 24 * time.Hour
	defaultThumbnailCacheEntries = 500
)

var defaultAllowedThumbnailHosts = []string{"img.youtube.com", "i.ytimg.com"}

// MediaHandler serves public media utility endpoints.
type MediaHandler struct {
	logger                  *slog.Logger
	cache                   *thumbnailCache
	skipSSRF                bool
	allowedThumbnailHosts   []string
	allowedThumbnailHostSet map[string]struct{}
	cacheTTL                time.Duration
	cacheMaxEntries         int
	fetchTimeout            time.Duration
	clock                   func() time.Time
	lookupIP                func(context.Context, string) ([]net.IP, error)
	dialAddress             func(context.Context, string, string, net.IP) (string, error)
}

// MediaHandlerOption configures optional MediaHandler settings.
type MediaHandlerOption func(*MediaHandler)

// WithSkipSSRF disables SSRF validation (for testing only).
func WithSkipSSRF() MediaHandlerOption {
	return func(h *MediaHandler) { h.skipSSRF = true }
}

// WithAllowedThumbnailHosts sets the host allowlist for public thumbnail fetches.
func WithAllowedThumbnailHosts(hosts ...string) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.allowedThumbnailHosts = append([]string(nil), hosts...)
	}
}

// WithAllowedHosts preserves the security-hardening option name while mapping
// to the thumbnail-specific allowlist.
func WithAllowedHosts(hosts ...string) MediaHandlerOption {
	return WithAllowedThumbnailHosts(hosts...)
}

// WithThumbnailCachePolicy sets the cache TTL and maximum number of entries.
func WithThumbnailCachePolicy(ttl time.Duration, maxEntries int) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.cacheTTL = ttl
		h.cacheMaxEntries = maxEntries
	}
}

// WithThumbnailFetchTimeout sets the upstream fetch timeout.
func WithThumbnailFetchTimeout(timeout time.Duration) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.fetchTimeout = timeout
	}
}

// WithLookupIPFunc overrides host resolution for SSRF validation and pinning.
func WithLookupIPFunc(fn func(context.Context, string) ([]net.IP, error)) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.lookupIP = fn
	}
}

// WithDialAddressFunc overrides how a pinned destination is translated into a dial address.
func WithDialAddressFunc(fn func(context.Context, string, string, net.IP) (string, error)) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.dialAddress = fn
	}
}

// WithClock injects a clock for deterministic TTL tests.
func WithClock(now func() time.Time) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.clock = now
	}
}

// NewMediaHandler creates a new MediaHandler.
func NewMediaHandler(logger *slog.Logger, opts ...MediaHandlerOption) *MediaHandler {
	h := &MediaHandler{
		logger:          logger,
		cacheTTL:        defaultThumbnailCacheTTL,
		cacheMaxEntries: defaultThumbnailCacheEntries,
		fetchTimeout:    defaultDownloadTimeout,
		clock:           time.Now,
		lookupIP: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		},
		dialAddress: func(_ context.Context, _ string, port string, pinnedIP net.IP) (string, error) {
			if pinnedIP == nil {
				return "", fmt.Errorf("missing pinned IP")
			}
			return net.JoinHostPort(pinnedIP.String(), port), nil
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	if len(h.allowedThumbnailHosts) == 0 {
		h.allowedThumbnailHosts = append([]string(nil), defaultAllowedThumbnailHosts...)
	}
	allowedSet := make(map[string]struct{}, len(h.allowedThumbnailHosts))
	for _, host := range h.allowedThumbnailHosts {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			allowedSet[host] = struct{}{}
		}
	}
	h.allowedThumbnailHostSet = allowedSet
	if h.cacheTTL <= 0 {
		h.cacheTTL = defaultThumbnailCacheTTL
	}
	if h.cacheMaxEntries <= 0 {
		h.cacheMaxEntries = defaultThumbnailCacheEntries
	}
	if h.fetchTimeout <= 0 {
		h.fetchTimeout = defaultDownloadTimeout
	}
	if h.clock == nil {
		h.clock = time.Now
	}
	h.cache = newThumbnailCache(h.cacheTTL, h.cacheMaxEntries, h.clock)
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
		"100.64.0.0/10",   // Carrier-grade NAT
		"192.0.0.0/24",    // IETF Protocol Assignments
		"192.0.2.0/24",    // Documentation
		"198.18.0.0/15",   // Benchmarking
		"198.51.100.0/24", // Documentation
		"203.0.113.0/24",  // Documentation
		"64:ff9b:1::/48",  // IPv6 NAT64 well-known prefix (local use / reserved)
		"100::/64",        // Discard-only / reserved
		"2001::/23",       // Teredo / infrastructure / reserved space
		"2001:db8::/32",   // Documentation
		"2001:10::/28",    // ORCHIDv2 / reserved
		"2002::/16",       // 6to4
		"fc00::/7",        // Unique local addresses
		"fe80::/10",       // Link-local unicast
		"0.0.0.0/8",       // Current network / special-purpose
		"240.0.0.0/4",     // Reserved
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

	if !h.isAllowedThumbnailHost(parsed.Hostname()) {
		return sendaresponse.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "url host is not allowlisted")
	}

	session := h.newFetchSession()
	if err := session.validateURL(c.Request().Context(), parsed); err != nil {
		if strings.Contains(err.Error(), "not allowlisted") {
			return sendaresponse.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "url host is not allowlisted")
		}
		if strings.Contains(err.Error(), "disallowed") {
			return sendaresponse.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "url resolves to a disallowed address")
		}
		return sendaresponse.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
	}

	// Serve from cache if available.
	if cached, ok := h.cache.Get(rawURL); ok {
		c.Response().Header().Set("Cache-Control", h.cacheControlHeader())
		return c.Blob(http.StatusOK, "image/png", cached)
	}

	// Download the thumbnail.
	pngBytes, err := h.buildComposite(c.Request().Context(), session, rawURL)
	if err != nil {
		h.logger.Warn("media: failed to build video thumbnail composite",
			"url", redactURL(rawURL),
			"error", err,
		)
		return sendaresponse.WriteError(c, http.StatusBadGateway, "BAD_GATEWAY", "could not retrieve thumbnail")
	}

	h.cache.Set(rawURL, pngBytes)

	c.Response().Header().Set("Cache-Control", h.cacheControlHeader())
	return c.Blob(http.StatusOK, "image/png", pngBytes)
}

func (h *MediaHandler) isAllowedThumbnailHost(host string) bool {
	if host == "" {
		return false
	}
	if h.allowedThumbnailHostSet == nil {
		return false
	}
	_, ok := h.allowedThumbnailHostSet[strings.ToLower(host)]
	return ok
}

func (h *MediaHandler) cacheControlHeader() string {
	seconds := int(h.cacheTTL.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("public, max-age=%d", seconds)
}

type mediaFetchSession struct {
	handler *MediaHandler
	pins    map[string]net.IP
	mu      sync.Mutex
	dialer  net.Dialer
}

func (h *MediaHandler) newFetchSession() *mediaFetchSession {
	return &mediaFetchSession{
		handler: h,
		pins:    make(map[string]net.IP),
		dialer:  net.Dialer{Timeout: h.fetchTimeout},
	}
}

func (s *mediaFetchSession) client() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = s.dialContext
	transport.MaxIdleConns = 0
	transport.IdleConnTimeout = 0

	return &http.Client{
		Timeout:   s.handler.fetchTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return s.validateURL(req.Context(), req.URL)
		},
	}
}

func (s *mediaFetchSession) validateURL(ctx context.Context, u *url.URL) error {
	if u == nil {
		return fmt.Errorf("thumbnail URL is nil")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("thumbnail URL must use http or https")
	}

	host := u.Hostname()
	if !s.handler.isAllowedThumbnailHost(host) {
		return fmt.Errorf("thumbnail host %q is not allowlisted", host)
	}

	_, err := s.ensurePinned(ctx, host)
	return err
}

func (s *mediaFetchSession) ensurePinned(ctx context.Context, host string) (net.IP, error) {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return nil, fmt.Errorf("thumbnail host is empty")
	}

	s.mu.Lock()
	if pinned, ok := s.pins[host]; ok {
		s.mu.Unlock()
		return cloneIP(pinned), nil
	}
	s.mu.Unlock()

	ips, err := s.handler.lookupIP(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolve host %q: no addresses returned", host)
	}

	chosen, err := choosePinnedIP(ips, s.handler.skipSSRF)
	if err != nil {
		return nil, err
	}

	pinned := cloneIP(chosen)
	s.mu.Lock()
	if existing, ok := s.pins[host]; ok {
		s.mu.Unlock()
		return cloneIP(existing), nil
	}
	s.pins[host] = pinned
	s.mu.Unlock()

	return cloneIP(pinned), nil
}

func (s *mediaFetchSession) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("split address %q: %w", addr, err)
	}

	pinnedIP, err := s.ensurePinned(ctx, host)
	if err != nil {
		return nil, err
	}

	dialAddr, err := s.handler.dialAddress(ctx, host, port, pinnedIP)
	if err != nil {
		return nil, err
	}

	return s.dialer.DialContext(ctx, network, dialAddr)
}

func cloneIP(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	return append(net.IP(nil), ip...)
}

func choosePinnedIP(ips []net.IP, skipSSRF bool) (net.IP, error) {
	if skipSSRF {
		for _, ip := range ips {
			if ip != nil {
				return ip, nil
			}
		}
		return nil, fmt.Errorf("no resolved addresses available")
	}

	var chosen net.IP
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		if isDisallowedIP(ip) {
			return nil, fmt.Errorf("url resolves to a disallowed address")
		}
		if chosen == nil {
			chosen = ip
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("no resolved addresses available")
	}
	return chosen, nil
}

func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[invalid-url]"
	}
	if parsed.Host == "" {
		return "[invalid-url]"
	}
	return parsed.Scheme + "://" + parsed.Host + parsed.Path
}

type thumbnailCache struct {
	mu         sync.Mutex
	items      map[string]*list.Element
	order      *list.List
	ttl        time.Duration
	maxEntries int
	now        func() time.Time
}

type thumbnailCacheEntry struct {
	key       string
	value     []byte
	createdAt time.Time
}

func newThumbnailCache(ttl time.Duration, maxEntries int, now func() time.Time) *thumbnailCache {
	if now == nil {
		now = time.Now
	}
	return &thumbnailCache{
		items:      make(map[string]*list.Element, maxEntries),
		order:      list.New(),
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        now,
	}
}

func (c *thumbnailCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	entry := elem.Value.(*thumbnailCacheEntry)
	if c.now().Sub(entry.createdAt) >= c.ttl {
		c.removeElement(elem)
		return nil, false
	}

	c.order.MoveToFront(elem)
	return append([]byte(nil), entry.value...), true
}

func (c *thumbnailCache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*thumbnailCacheEntry)
		entry.value = append(entry.value[:0], value...)
		entry.createdAt = c.now()
		c.order.MoveToFront(elem)
		return
	}

	if c.order.Len() >= c.maxEntries {
		back := c.order.Back()
		if back != nil {
			c.removeElement(back)
		}
	}

	elem := c.order.PushFront(&thumbnailCacheEntry{
		key:       key,
		value:     append([]byte(nil), value...),
		createdAt: c.now(),
	})
	c.items[key] = elem
}

func (c *thumbnailCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*thumbnailCacheEntry)
	delete(c.items, entry.key)
	c.order.Remove(elem)
}

// buildComposite downloads the image at rawURL, draws the play-button overlay,
// and returns the result encoded as PNG bytes.
func (h *MediaHandler) buildComposite(ctx context.Context, session *mediaFetchSession, rawURL string) ([]byte, error) {
	candidates := thumbnailCandidates(rawURL)
	var lastErr error
	for _, candidate := range candidates {
		pngBytes, err := h.buildCompositeForURL(ctx, session, candidate)
		if err == nil {
			return pngBytes, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("thumbnail could not be generated")
	}
	return nil, lastErr
}

func (h *MediaHandler) buildCompositeForURL(ctx context.Context, session *mediaFetchSession, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse thumbnail URL %q: %w", redactURL(rawURL), err)
	}
	if err := session.validateURL(ctx, parsed); err != nil {
		return nil, err
	}

	client := session.client()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", redactURL(rawURL), err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", redactURL(rawURL), err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: upstream returned %d", redactURL(rawURL), resp.StatusCode)
	}

	// Guard against oversized images.
	limited := io.LimitReader(resp.Body, maxDownloadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body %s: %w", redactURL(rawURL), err)
	}
	if len(data) > maxDownloadBytes {
		return nil, fmt.Errorf("thumbnail exceeds 10 MB limit")
	}

	// Decode the image (JPEG and PNG are the common thumbnail formats).
	src, err := decodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("decode image %s: %w", redactURL(rawURL), err)
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

var youtubeThumbPath = regexp.MustCompile(`^/vi/([A-Za-z0-9_-]{11})/[^/]+$`)

func thumbnailCandidates(rawURL string) []string {
	candidates := []string{rawURL}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return candidates
	}

	matches := youtubeThumbPath.FindStringSubmatch(parsed.Path)
	if len(matches) != 2 {
		return candidates
	}

	videoID := matches[1]
	base := &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   path.Clean(path.Join("/vi", videoID)),
	}

	for _, variant := range []string{"hqdefault.jpg", "mqdefault.jpg", "default.jpg"} {
		next := *base
		next.Path = path.Join(base.Path, variant)
		candidates = append(candidates, next.String())
	}

	return candidates
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

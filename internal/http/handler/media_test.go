package handler_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5/echotest"
	"github.com/rendis/senda/internal/http/handler"
)

// makeTestJPEG returns raw JPEG bytes for a solid-colour 200×150 image.
func makeTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 150))
	red := color.RGBA{R: 200, G: 50, B: 50, A: 255}
	for y := 0; y < 150; y++ {
		for x := 0; x < 200; x++ {
			img.SetRGBA(x, y, red)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("makeTestJPEG: %v", err)
	}
	return buf.Bytes()
}

// makeTestPNG returns raw PNG bytes for a solid-colour 100×100 image.
func makeTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	blue := color.RGBA{R: 30, G: 80, B: 200, A: 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, blue)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("makeTestPNG: %v", err)
	}
	return buf.Bytes()
}

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func mediaHandlerForURLHost(t *testing.T, rawURL string, opts ...handler.MediaHandlerOption) *handler.MediaHandler {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse host for allowlist: %v", err)
	}
	opts = append([]handler.MediaHandlerOption{handler.WithAllowedThumbnailHosts(parsed.Hostname())}, opts...)
	return handler.NewMediaHandler(newSilentLogger(), opts...)
}

func newPinnedMediaHandler(t *testing.T, lookup func(context.Context, string) ([]net.IP, error), dial func(context.Context, string, string, net.IP) (string, error), hosts ...string) *handler.MediaHandler {
	t.Helper()
	opts := []handler.MediaHandlerOption{
		handler.WithAllowedHosts(hosts...),
		handler.WithLookupIPFunc(lookup),
		handler.WithDialAddressFunc(dial),
	}
	return handler.NewMediaHandler(newSilentLogger(), opts...)
}

// TestHandleVideoThumbnail_MissingURL verifies that omitting the url parameter
// returns 400 Bad Request.
func TestHandleVideoThumbnail_MissingURL(t *testing.T) {
	h := handler.NewMediaHandler(newSilentLogger())

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/public/video-thumbnail", nil),
	}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleVideoThumbnail_DenyByDefaultWithoutAllowlist(t *testing.T) {
	h := handler.NewMediaHandler(newSilentLogger(), handler.WithSkipSSRF())

	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("http://127.0.0.1/thumb.jpg"), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 without allowlist, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleVideoThumbnail_InvalidScheme verifies that a non-http/https URL
// (e.g. ftp://) is rejected with 400 Bad Request.
func TestHandleVideoThumbnail_InvalidScheme(t *testing.T) {
	h := handler.NewMediaHandler(newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("ftp://example.com/thumb.jpg"), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleVideoThumbnail_InvalidScheme_File verifies that a file:// URL is
// rejected with 400 Bad Request (security: must not read local files).
func TestHandleVideoThumbnail_InvalidScheme_File(t *testing.T) {
	h := handler.NewMediaHandler(newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("file:///etc/passwd"), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleVideoThumbnail_Success_JPEG starts a local HTTP server that serves a
// JPEG thumbnail, calls the handler, and verifies:
//   - HTTP 200
//   - Content-Type: image/png
//   - Cache-Control header set correctly
//   - Response body is valid PNG
func TestHandleVideoThumbnail_Success_JPEG(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	// Spin up a local thumbnail server.
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))
	defer thumbServer.Close()

	thumbURL := thumbServer.URL + "/thumb.jpg"
	h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %q", ct)
	}

	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=86400" {
		t.Fatalf("expected Cache-Control 'public, max-age=86400', got %q", cc)
	}

	// Verify that the response body is a valid PNG.
	img, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatalf("response is not a valid PNG: %v", err)
	}

	// Sanity-check dimensions: output should match the source thumbnail.
	bounds := img.Bounds()
	if bounds.Dx() != 200 || bounds.Dy() != 150 {
		t.Fatalf("expected 200×150 PNG, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// TestHandleVideoThumbnail_Success_PNG verifies that a PNG thumbnail is also
// processed correctly and returned as a composite PNG.
func TestHandleVideoThumbnail_Success_PNG(t *testing.T) {
	pngBytes := makeTestPNG(t)

	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBytes)
	}))
	defer thumbServer.Close()

	thumbURL := thumbServer.URL + "/thumb.png"
	h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %q", rec.Header().Get("Content-Type"))
	}

	if _, err := png.Decode(rec.Body); err != nil {
		t.Fatalf("response is not valid PNG: %v", err)
	}
}

// TestHandleVideoThumbnail_YouTubeFallback verifies that when the preferred
// YouTube maxres thumbnail does not exist, the handler falls back to a lower
// quality variant instead of failing the preview.
func TestHandleVideoThumbnail_YouTubeFallback(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	var reqCount int
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++

		switch r.URL.Path {
		case "/vi/dQw4w9WgXcQ/maxresdefault.jpg":
			http.NotFound(w, r)
		case "/vi/dQw4w9WgXcQ/hqdefault.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jpegBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer thumbServer.Close()

	thumbURL := thumbServer.URL + "/vi/dQw4w9WgXcQ/maxresdefault.jpg"
	h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %q", rec.Header().Get("Content-Type"))
	}

	if _, err := png.Decode(rec.Body); err != nil {
		t.Fatalf("response is not valid PNG: %v", err)
	}

	if reqCount != 2 {
		t.Fatalf("expected fallback to request 2 URLs, got %d", reqCount)
	}
}

// TestHandleVideoThumbnail_UpstreamError verifies that when the upstream server
// returns a non-2xx status the handler returns 502 Bad Gateway.
func TestHandleVideoThumbnail_UpstreamError(t *testing.T) {
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer thumbServer.Close()

	thumbURL := thumbServer.URL + "/missing.jpg"
	h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleVideoThumbnail_RejectsHostNotInAllowlist verifies the explicit
// allowlist blocks unrelated hosts even when SSRF checks are bypassed.
func TestHandleVideoThumbnail_RejectsHostNotInAllowlist(t *testing.T) {
	h := handler.NewMediaHandler(
		newSilentLogger(),
		handler.WithSkipSSRF(),
		handler.WithAllowedThumbnailHosts("allowed.example"),
	)

	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("https://blocked.example/thumb.jpg"), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for non-allowlisted host, got %d", rec.Code)
	}
}

// TestHandleVideoThumbnail_RejectsRedirectToNonAllowlistedHost verifies that
// each redirect hop is checked against the allowlist, not only the initial URL.
func TestHandleVideoThumbnail_RejectsRedirectToNonAllowlistedHost(t *testing.T) {
	originalDefaultTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalDefaultTransport
	})

	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(makeTestJPEG(t))
	}))
	defer redirectTarget.Close()

	targetAddr := redirectTarget.Listener.Addr().String()
	allowlistedURL, err := url.Parse(redirectTarget.URL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}

	baseTransport := originalDefaultTransport.(*http.Transport).Clone()
	baseTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if host, _, splitErr := net.SplitHostPort(addr); splitErr == nil && host == "blocked.example" {
			addr = targetAddr
		}
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, addr)
	}
	http.DefaultTransport = baseTransport

	var targetReqCount int
	redirectTarget.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetReqCount++
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(makeTestJPEG(t))
	})

	redirectSource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://blocked.example:"+allowlistedURL.Port()+"/thumb.jpg", http.StatusFound)
	}))
	defer redirectSource.Close()

	h := handler.NewMediaHandler(
		newSilentLogger(),
		handler.WithSkipSSRF(),
		handler.WithAllowedThumbnailHosts(urlMustHost(t, redirectSource.URL+"/thumb.jpg")),
	)

	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(redirectSource.URL+"/thumb.jpg"), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502 for redirect to non-allowlisted host, got %d — body: %s", rec.Code, rec.Body.String())
	}

	if targetReqCount != 0 {
		t.Fatalf("expected redirect target to remain unrequested, got %d requests", targetReqCount)
	}
}

// TestHandleVideoThumbnail_CacheHit verifies that the second request for the
// same URL is served from cache (the upstream server is shut down before the
// second request).

func TestHandleVideoThumbnail_CacheHit_PreservesHeadersAndBody(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	var reqCount int
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))

	thumbURL := thumbServer.URL + "/thumb.jpg"
	h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())

	req1 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c1, rec1 := echotest.ContextConfig{Request: req1}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c1); err != nil {
		t.Fatalf("first request: unexpected error: %v", err)
	}
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	firstHeaders := rec1.Header().Clone()
	firstBody := append([]byte(nil), rec1.Body.Bytes()...)

	thumbServer.Close()

	req2 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c2, rec2 := echotest.ContextConfig{Request: req2}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c2); err != nil {
		t.Fatalf("second request: unexpected error: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d — body: %s", rec2.Code, rec2.Body.String())
	}

	if got := rec2.Header().Get("Content-Type"); got != firstHeaders.Get("Content-Type") {
		t.Fatalf("expected cached Content-Type %q, got %q", firstHeaders.Get("Content-Type"), got)
	}
	if got := rec2.Header().Get("Cache-Control"); got != firstHeaders.Get("Cache-Control") {
		t.Fatalf("expected cached Cache-Control %q, got %q", firstHeaders.Get("Cache-Control"), got)
	}
	if !bytes.Equal(rec2.Body.Bytes(), firstBody) {
		t.Fatalf("expected cached body to match original response exactly")
	}
	if reqCount != 1 {
		t.Fatalf("expected exactly 1 upstream request, got %d", reqCount)
	}
}

func TestHandleVideoThumbnail_Pinning_IsScopedPerRequest(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))
	defer thumbServer.Close()

	var mu sync.Mutex
	lookupCount := 0
	pinnedByRequest := []string{}
	lookup := func(_ context.Context, host string) ([]net.IP, error) {
		mu.Lock()
		defer mu.Unlock()
		lookupCount++
		if host != "media.example.test" {
			return nil, fmt.Errorf("unexpected host %q", host)
		}
		if lookupCount == 1 {
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		}
		return []net.IP{net.ParseIP("8.8.4.4")}, nil
	}

	dial := func(_ context.Context, host, port string, pinnedIP net.IP) (string, error) {
		mu.Lock()
		pinnedByRequest = append(pinnedByRequest, pinnedIP.String())
		mu.Unlock()
		if host != "media.example.test" {
			return "", fmt.Errorf("unexpected dial host %q", host)
		}
		return thumbServer.Listener.Addr().String(), nil
	}

	h := newPinnedMediaHandler(t, lookup, dial, "media.example.test")

	for i := 0; i < 2; i++ {
		rawURL := fmt.Sprintf("http://media.example.test/thumb-%d.jpg", i+1)
		req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(rawURL), nil)
		c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)
		if err := h.HandleVideoThumbnail(c); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i+1, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d — body: %s", i+1, rec.Code, rec.Body.String())
		}
	}

	if lookupCount != 2 {
		t.Fatalf("expected one fresh lookup per request, got %d lookups", lookupCount)
	}
	wantPins := []string{"8.8.8.8", "8.8.4.4"}
	if fmt.Sprintf("%v", pinnedByRequest) != fmt.Sprintf("%v", wantPins) {
		t.Fatalf("expected request-scoped pins %v, got %v", wantPins, pinnedByRequest)
	}
}

func TestHandleVideoThumbnail_InvalidOrOversizedImage_RemainsBadGateway(t *testing.T) {
	overLimit := bytes.Repeat([]byte("a"), 10*1024*1024+1)
	tests := []struct {
		name string
		body []byte
	}{
		{name: "invalid image", body: []byte("not-an-image")},
		{name: "oversized image", body: overLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/jpeg")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tt.body)
			}))
			defer thumbServer.Close()

			thumbURL := thumbServer.URL + "/thumb.jpg"
			h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())
			req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
			c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

			if err := h.HandleVideoThumbnail(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("expected 502, got %d — body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleVideoThumbnail_CacheHit(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	var reqCount int
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))

	thumbURL := thumbServer.URL + "/thumb.jpg"
	h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())

	// First request — populates cache.
	req1 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c1, rec1 := echotest.ContextConfig{Request: req1}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c1); err != nil {
		t.Fatalf("first request: unexpected error: %v", err)
	}
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	// Close the upstream; any further HTTP request to it will fail.
	thumbServer.Close()

	// Second request — must be served from cache without hitting the server.
	req2 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c2, rec2 := echotest.ContextConfig{Request: req2}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c2); err != nil {
		t.Fatalf("second request: unexpected error: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d — body: %s", rec2.Code, rec2.Body.String())
	}

	if reqCount != 1 {
		t.Fatalf("expected exactly 1 upstream request (cache hit for second), got %d", reqCount)
	}
}

// TestHandleVideoThumbnail_CacheExpiresAfterTTL verifies that cached thumbnails
// expire once the configured TTL elapses.
func TestHandleVideoThumbnail_CacheExpiresAfterTTL(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	var reqCount int
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))
	defer thumbServer.Close()

	thumbURL := thumbServer.URL + "/thumb.jpg"
	now := time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC)
	h := handler.NewMediaHandler(
		newSilentLogger(),
		handler.WithSkipSSRF(),
		handler.WithAllowedThumbnailHosts(urlMustHost(t, thumbURL)),
		handler.WithThumbnailCachePolicy(time.Minute, 4),
		handler.WithClock(func() time.Time { return now }),
	)

	req1 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c1, rec1 := echotest.ContextConfig{Request: req1}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c1); err != nil {
		t.Fatalf("first request: unexpected error: %v", err)
	}
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	now = now.Add(2 * time.Minute)

	req2 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c2, rec2 := echotest.ContextConfig{Request: req2}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c2); err != nil {
		t.Fatalf("second request: unexpected error: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", rec2.Code)
	}

	if reqCount != 2 {
		t.Fatalf("expected cache expiry to trigger a second upstream request, got %d", reqCount)
	}
}

// TestHandleVideoThumbnail_EvictsOldestEntryWhenCacheFull verifies that the
// cache respects its maximum entry count and evicts the oldest thumbnail.
func TestHandleVideoThumbnail_EvictsOldestEntryWhenCacheFull(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	var reqCount int
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))
	defer thumbServer.Close()

	allowedHost := urlMustHost(t, thumbServer.URL+"/one.jpg")
	h := handler.NewMediaHandler(
		newSilentLogger(),
		handler.WithSkipSSRF(),
		handler.WithAllowedThumbnailHosts(allowedHost),
		handler.WithThumbnailCachePolicy(time.Hour, 1),
	)

	url1 := thumbServer.URL + "/one.jpg"
	url2 := thumbServer.URL + "/two.jpg"

	req1 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(url1), nil)
	c1, rec1 := echotest.ContextConfig{Request: req1}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c1); err != nil {
		t.Fatalf("first request: unexpected error: %v", err)
	}
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(url2), nil)
	c2, rec2 := echotest.ContextConfig{Request: req2}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c2); err != nil {
		t.Fatalf("second request: unexpected error: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", rec2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(url1), nil)
	c3, rec3 := echotest.ContextConfig{Request: req3}.ToContextRecorder(t)
	if err := h.HandleVideoThumbnail(c3); err != nil {
		t.Fatalf("third request: unexpected error: %v", err)
	}
	if rec3.Code != http.StatusOK {
		t.Fatalf("third request: expected 200, got %d", rec3.Code)
	}

	if reqCount != 3 {
		t.Fatalf("expected eviction to force a third upstream request, got %d", reqCount)
	}
}

func TestHandleVideoThumbnail_RedirectToPrivateDestinationRejected(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect.jpg":
			http.Redirect(w, r, "http://cdn.example.test/final.jpg", http.StatusFound)
		case "/final.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jpegBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer thumbServer.Close()

	var lookupCalls []string
	lookup := func(_ context.Context, host string) ([]net.IP, error) {
		lookupCalls = append(lookupCalls, host)
		switch host {
		case "media.example.test":
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		case "cdn.example.test":
			return []net.IP{net.ParseIP("10.0.0.1")}, nil
		default:
			return nil, fmt.Errorf("unexpected host %q", host)
		}
	}

	dial := func(_ context.Context, host, port string, pinnedIP net.IP) (string, error) {
		if host != "media.example.test" {
			return "", fmt.Errorf("unexpected dial host %q", host)
		}
		if pinnedIP == nil || pinnedIP.String() != "8.8.8.8" {
			return "", fmt.Errorf("unexpected pinned ip %v", pinnedIP)
		}
		return thumbServer.Listener.Addr().String(), nil
	}

	h := newPinnedMediaHandler(t, lookup, dial, "media.example.test", "cdn.example.test")

	reqURL := "http://media.example.test/redirect.jpg"
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(reqURL), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for redirect to private destination, got %d", rec.Code)
	}
	if len(lookupCalls) != 2 || strings.Join(lookupCalls, ",") != "media.example.test,cdn.example.test" {
		t.Fatalf("expected both hosts to be evaluated, got %v", lookupCalls)
	}
}

func TestHandleVideoThumbnail_PinsResolvedDestinationAcrossRedirects(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect.jpg":
			http.Redirect(w, r, "http://media.example.test/final.jpg", http.StatusFound)
		case "/final.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jpegBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer thumbServer.Close()

	lookupCount := map[string]int{}
	lookup := func(_ context.Context, host string) ([]net.IP, error) {
		lookupCount[host]++
		if host != "media.example.test" {
			return nil, fmt.Errorf("unexpected host %q", host)
		}
		if lookupCount[host] == 1 {
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		}
		return []net.IP{net.ParseIP("10.0.0.1")}, nil
	}

	dial := func(_ context.Context, host, port string, pinnedIP net.IP) (string, error) {
		if host != "media.example.test" {
			return "", fmt.Errorf("unexpected dial host %q", host)
		}
		if pinnedIP == nil || pinnedIP.String() != "8.8.8.8" {
			return "", fmt.Errorf("unexpected pinned ip %v", pinnedIP)
		}
		return thumbServer.Listener.Addr().String(), nil
	}

	h := newPinnedMediaHandler(t, lookup, dial, "media.example.test")

	reqURL := "http://media.example.test/redirect.jpg"
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(reqURL), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for pinned redirect flow, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if lookupCount["media.example.test"] != 1 {
		t.Fatalf("expected lookup to be pinned after first resolution, got %d lookups", lookupCount["media.example.test"])
	}
}

func TestHandleVideoThumbnail_MixedDNSAnswersFailClosed(t *testing.T) {
	tests := []struct {
		name string
		ips  []net.IP
	}{
		{name: "mixed_ipv4_private", ips: []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.1")}},
		{name: "mixed_ipv6_reserved", ips: []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("fc00::1")}},
		{name: "mixed_ipv4_special_purpose", ips: []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("192.0.2.1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookupCalls := 0
			lookup := func(_ context.Context, host string) ([]net.IP, error) {
				lookupCalls++
				if host != "media.example.test" {
					return nil, fmt.Errorf("unexpected host %q", host)
				}
				return tt.ips, nil
			}

			dial := func(_ context.Context, host, port string, pinnedIP net.IP) (string, error) {
				t.Fatalf("dial must not be reached for mixed DNS answers: host=%s port=%s ip=%v", host, port, pinnedIP)
				return "", nil
			}

			h := newPinnedMediaHandler(t, lookup, dial, "media.example.test")
			req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("http://media.example.test/redirect.jpg"), nil)
			c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

			if err := h.HandleVideoThumbnail(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for mixed DNS answers, got %d — body: %s", rec.Code, rec.Body.String())
			}
			if lookupCalls != 1 {
				t.Fatalf("expected a single lookup, got %d", lookupCalls)
			}
		})
	}
}

func TestHandleVideoThumbnail_SSRFSpecialPurposeIPv4Blocked(t *testing.T) {
	tests := []struct {
		name string
		host string
		ip   string
	}{
		{name: "zero_net", host: "0.0.0.1", ip: "0.0.0.1"},
		{name: "documentation_net", host: "192.0.2.1", ip: "192.0.2.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(_ context.Context, host string) ([]net.IP, error) {
				if host != tt.host {
					return nil, fmt.Errorf("unexpected host %q", host)
				}
				return []net.IP{net.ParseIP(tt.ip)}, nil
			}
			dial := func(_ context.Context, host, port string, pinnedIP net.IP) (string, error) {
				t.Fatalf("dial must not be reached for special-purpose IPv4: host=%s port=%s ip=%v", host, port, pinnedIP)
				return "", nil
			}

			h := newPinnedMediaHandler(t, lookup, dial, tt.host)
			req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("http://"+tt.host+"/thumb.jpg"), nil)
			c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

			if err := h.HandleVideoThumbnail(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for special-purpose IPv4 %q, got %d — body: %s", tt.host, rec.Code, rec.Body.String())
			}
		})
	}
}

func urlMustHost(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return parsed.Hostname()
}

// TestHandleVideoThumbnail_SSRFBlocked verifies that URLs resolving to private
// or loopback addresses are rejected with 400 Bad Request.
func TestHandleVideoThumbnail_SSRFBlocked(t *testing.T) {
	h := handler.NewMediaHandler(newSilentLogger())

	ssrfURLs := []string{
		"http://127.0.0.1/thumb.jpg",
		"http://localhost/thumb.jpg",
		"http://[::1]/thumb.jpg",
		"http://192.168.1.1/thumb.jpg",
		"http://10.0.0.1/thumb.jpg",
		"http://172.16.0.1/thumb.jpg",
	}

	for _, rawURL := range ssrfURLs {
		rawURL := rawURL // capture range variable
		t.Run(rawURL, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(rawURL), nil)
			c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

			if err := h.HandleVideoThumbnail(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for SSRF URL %q, got %d — body: %s", rawURL, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestHandleVideoThumbnail_ConcurrentSameURL fires 10 goroutines all requesting
// the same thumbnail URL simultaneously and verifies all receive 200 + image/png.
// The test is safe to run with -race.
func TestHandleVideoThumbnail_ConcurrentSameURL(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))
	defer thumbServer.Close()

	thumbURL := thumbServer.URL + "/thumb.jpg"
	h := mediaHandlerForURLHost(t, thumbURL, handler.WithSkipSSRF())

	const goroutines = 10

	type result struct {
		code        int
		contentType string
		err         error
	}

	results := make([]result, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i // capture range variable
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
			c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

			handlerErr := h.HandleVideoThumbnail(c)
			results[i] = result{
				code:        rec.Code,
				contentType: rec.Header().Get("Content-Type"),
				err:         handlerErr,
			}
		}()
	}

	wg.Wait()

	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d: unexpected handler error: %v", i, r.err)
			continue
		}
		if r.code != http.StatusOK {
			t.Errorf("goroutine %d: expected 200, got %d", i, r.code)
		}
		if r.contentType != "image/png" {
			t.Errorf("goroutine %d: expected Content-Type image/png, got %q", i, r.contentType)
		}
	}
}

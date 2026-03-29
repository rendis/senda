package handler_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

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

	h := handler.NewMediaHandler(newSilentLogger(), handler.WithSkipSSRF())

	thumbURL := thumbServer.URL + "/thumb.jpg"
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

	h := handler.NewMediaHandler(newSilentLogger(), handler.WithSkipSSRF())

	thumbURL := thumbServer.URL + "/thumb.png"
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

// TestHandleVideoThumbnail_UpstreamError verifies that when the upstream server
// returns a non-2xx status the handler returns 502 Bad Gateway.
func TestHandleVideoThumbnail_UpstreamError(t *testing.T) {
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer thumbServer.Close()

	h := handler.NewMediaHandler(newSilentLogger(), handler.WithSkipSSRF())

	thumbURL := thumbServer.URL + "/missing.jpg"
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape(thumbURL), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleVideoThumbnail_CacheHit verifies that the second request for the
// same URL is served from cache (the upstream server is shut down before the
// second request).
func TestHandleVideoThumbnail_CacheHit(t *testing.T) {
	jpegBytes := makeTestJPEG(t)

	var reqCount int
	thumbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jpegBytes)
	}))

	h := handler.NewMediaHandler(newSilentLogger(), handler.WithSkipSSRF())
	thumbURL := thumbServer.URL + "/thumb.jpg"

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

	h := handler.NewMediaHandler(newSilentLogger(), handler.WithSkipSSRF())
	thumbURL := thumbServer.URL + "/thumb.jpg"

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

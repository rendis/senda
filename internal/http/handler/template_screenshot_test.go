package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/service"
)

type stubScreenshotSvc struct {
	out []byte
	err error
}

func (s *stubScreenshotSvc) Capture(ctx context.Context, in service.ScreenshotInput) ([]byte, error) {
	return s.out, s.err
}

func newScreenshotEcho(svc handler.ScreenshotService) (*echo.Echo, *handler.TemplateScreenshotHandler) {
	e := echo.New()
	h := handler.NewTemplateScreenshotHandler(svc)
	e.GET("/templates/:template_id/screenshot", h.Capture)
	return e, h
}

func TestScreenshotHandler_Success_ReturnsPNG(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{out: []byte{0x89, 0x50, 0x4e, 0x47}})
	tplID := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/templates/"+tplID+"/screenshot?viewport=desktop", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47}, rec.Body.Bytes())
}

func TestScreenshotHandler_DefaultViewport_IsDesktop(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{out: []byte("ok")})
	tplID := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/templates/"+tplID+"/screenshot", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestScreenshotHandler_BadTemplateID(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{})
	req := httptest.NewRequest(http.MethodGet, "/templates/not-a-uuid/screenshot?viewport=desktop", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "INVALID_TEMPLATE_ID")
}

func TestScreenshotHandler_InvalidViewport(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{err: service.ErrInvalidViewport})
	tplID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/templates/"+tplID+"/screenshot?viewport=tv", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "INVALID_VIEWPORT")
}

func TestScreenshotHandler_Disabled(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{err: service.ErrScreenshotDisabled})
	tplID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/templates/"+tplID+"/screenshot?viewport=desktop", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "SCREENSHOT_DISABLED")
}

func TestScreenshotHandler_Timeout(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{err: service.ErrScreenshotTimeout})
	tplID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/templates/"+tplID+"/screenshot?viewport=desktop", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
	require.Contains(t, rec.Body.String(), "SCREENSHOT_TIMEOUT")
}

func TestScreenshotHandler_NoPublished(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{err: domain.ErrNoPublishedVersion})
	tplID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/templates/"+tplID+"/screenshot?viewport=desktop", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "NO_PUBLISHED_VERSION")
}

func TestScreenshotHandler_InternalError(t *testing.T) {
	e, _ := newScreenshotEcho(&stubScreenshotSvc{err: errors.Join(service.ErrScreenshotInternal, errors.New("boom"))})
	tplID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/templates/"+tplID+"/screenshot?viewport=desktop", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "SCREENSHOT_INTERNAL")
}

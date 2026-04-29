package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// --- Manual mocks ---

type stubTemplateStore struct {
	tpl       *domain.Template
	tplErr    error
	published *domain.TemplateVersion
	pubErr    error
	versions  map[uuid.UUID]*domain.TemplateVersion
	locales   map[string]*domain.TemplateVersionLocale
	verErr    error
	locErr    error
}

func (s *stubTemplateStore) GetTemplateByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	return s.tpl, s.tplErr
}
func (s *stubTemplateStore) GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
	return s.published, s.pubErr
}
func (s *stubTemplateStore) GetVersionByID(ctx context.Context, id uuid.UUID) (*domain.TemplateVersion, error) {
	if s.verErr != nil {
		return nil, s.verErr
	}
	return s.versions[id], nil
}
func (s *stubTemplateStore) GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
	if s.locErr != nil {
		return nil, s.locErr
	}
	return s.locales[locale], nil
}

type stubCompiler struct {
	html string
	err  error
}

func (c *stubCompiler) Compile(ctx context.Context, mjml string) (string, error) {
	return c.html, c.err
}

type stubCapturer struct {
	gotHTML string
	gotVP   port.Viewport
	gotCap  int
	out     []byte
	err     error
}

func (c *stubCapturer) Capture(ctx context.Context, html string, vp port.Viewport, maxHeight int) ([]byte, error) {
	c.gotHTML = html
	c.gotVP = vp
	c.gotCap = maxHeight
	return c.out, c.err
}

// --- Tests ---

func TestScreenshot_LatestPublished_Desktop(t *testing.T) {
	tplID := uuid.New()
	verID := uuid.New()
	store := &stubTemplateStore{
		tpl:       &domain.Template{ID: tplID, IsDisabled: false},
		published: &domain.TemplateVersion{ID: verID, BodyMJML: "<mjml>x</mjml>", DefaultLocale: "en"},
	}
	compiler := &stubCompiler{html: "<html>compiled</html>"}
	cap := &stubCapturer{out: []byte{0x89, 0x50, 0x4e, 0x47}}
	cfg := config.ScreenshotConfig{Enabled: true, MaxHeightPx: 6000, DesktopWidthPx: 1280, MobileWidthPx: 390}

	svc := service.NewTemplateScreenshotService(store, compiler, cap, cfg)

	out, err := svc.Capture(context.Background(), service.ScreenshotInput{
		TemplateID: tplID, Viewport: "desktop",
	})
	require.NoError(t, err)
	require.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47}, out)
	require.Equal(t, "<html>compiled</html>", cap.gotHTML)
	require.Equal(t, "desktop", cap.gotVP.Name)
	require.Equal(t, 1280, cap.gotVP.WidthPx)
	require.False(t, cap.gotVP.MobileEmul)
	require.Equal(t, 6000, cap.gotCap)
}

func TestScreenshot_Mobile_UsesMobilePreset(t *testing.T) {
	store := &stubTemplateStore{
		tpl:       &domain.Template{ID: uuid.New()},
		published: &domain.TemplateVersion{ID: uuid.New(), BodyMJML: "<mjml/>", DefaultLocale: "en"},
	}
	compiler := &stubCompiler{html: "<html/>"}
	cap := &stubCapturer{out: []byte("png")}
	cfg := config.ScreenshotConfig{Enabled: true, MaxHeightPx: 6000, DesktopWidthPx: 1280, MobileWidthPx: 390}
	svc := service.NewTemplateScreenshotService(store, compiler, cap, cfg)

	_, err := svc.Capture(context.Background(), service.ScreenshotInput{
		TemplateID: store.tpl.ID, Viewport: "mobile",
	})
	require.NoError(t, err)
	require.Equal(t, "mobile", cap.gotVP.Name)
	require.Equal(t, 390, cap.gotVP.WidthPx)
	require.True(t, cap.gotVP.MobileEmul)
	require.Equal(t, 2.0, cap.gotVP.DeviceScale)
}

func TestScreenshot_NoPublished_ReturnsErrNoPublishedVersion(t *testing.T) {
	store := &stubTemplateStore{
		tpl:    &domain.Template{ID: uuid.New()},
		pubErr: domain.ErrNoPublishedVersion,
	}
	cfg := config.ScreenshotConfig{Enabled: true, MaxHeightPx: 6000, DesktopWidthPx: 1280}
	svc := service.NewTemplateScreenshotService(store, &stubCompiler{}, &stubCapturer{}, cfg)

	_, err := svc.Capture(context.Background(), service.ScreenshotInput{
		TemplateID: store.tpl.ID, Viewport: "desktop",
	})
	require.ErrorIs(t, err, domain.ErrNoPublishedVersion)
}

func TestScreenshot_TemplateDisabled_ReturnsConflict(t *testing.T) {
	store := &stubTemplateStore{
		tpl: &domain.Template{ID: uuid.New(), IsDisabled: true},
	}
	cfg := config.ScreenshotConfig{Enabled: true, MaxHeightPx: 6000, DesktopWidthPx: 1280}
	svc := service.NewTemplateScreenshotService(store, &stubCompiler{}, &stubCapturer{}, cfg)

	_, err := svc.Capture(context.Background(), service.ScreenshotInput{
		TemplateID: store.tpl.ID, Viewport: "desktop",
	})
	require.ErrorIs(t, err, domain.ErrTemplateDisabled)
}

func TestScreenshot_LocaleOverride(t *testing.T) {
	tplID := uuid.New()
	verID := uuid.New()
	esBody := "es mjml"
	store := &stubTemplateStore{
		tpl:       &domain.Template{ID: tplID},
		published: &domain.TemplateVersion{ID: verID, BodyMJML: "default mjml", DefaultLocale: "en"},
		locales: map[string]*domain.TemplateVersionLocale{
			"es": {TemplateVersionID: verID, Locale: "es", BodyMJML: &esBody},
		},
	}
	compiler := &stubCompiler{html: "<html/>"}
	cap := &stubCapturer{out: []byte("png")}
	cfg := config.ScreenshotConfig{Enabled: true, MaxHeightPx: 6000, DesktopWidthPx: 1280, MobileWidthPx: 390}
	svc := service.NewTemplateScreenshotService(store, compiler, cap, cfg)

	_, err := svc.Capture(context.Background(), service.ScreenshotInput{
		TemplateID: tplID, Viewport: "desktop", Locale: "es",
	})
	require.NoError(t, err)
}

func TestScreenshot_Disabled_ReturnsErrDisabled(t *testing.T) {
	cfg := config.ScreenshotConfig{Enabled: false}
	svc := service.NewTemplateScreenshotService(nil, nil, nil, cfg)

	_, err := svc.Capture(context.Background(), service.ScreenshotInput{Viewport: "desktop"})
	require.ErrorIs(t, err, service.ErrScreenshotDisabled)
}

func TestScreenshot_InvalidViewport(t *testing.T) {
	cfg := config.ScreenshotConfig{Enabled: true, MaxHeightPx: 6000, DesktopWidthPx: 1280, MobileWidthPx: 390}
	svc := service.NewTemplateScreenshotService(nil, nil, nil, cfg)

	_, err := svc.Capture(context.Background(), service.ScreenshotInput{Viewport: "tablet"})
	require.ErrorIs(t, err, service.ErrInvalidViewport)
}

func TestScreenshot_CapturerError_PropagatesAsInternal(t *testing.T) {
	store := &stubTemplateStore{
		tpl:       &domain.Template{ID: uuid.New()},
		published: &domain.TemplateVersion{ID: uuid.New(), BodyMJML: "<mjml/>", DefaultLocale: "en"},
	}
	cap := &stubCapturer{err: errors.New("browser crashed")}
	cfg := config.ScreenshotConfig{Enabled: true, MaxHeightPx: 6000, DesktopWidthPx: 1280}
	svc := service.NewTemplateScreenshotService(store, &stubCompiler{html: "<html/>"}, cap, cfg)

	_, err := svc.Capture(context.Background(), service.ScreenshotInput{
		TemplateID: store.tpl.ID, Viewport: "desktop",
	})
	require.ErrorIs(t, err, service.ErrScreenshotInternal)
}

package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// Sentinel errors surfaced by the service layer.
var (
	ErrScreenshotDisabled = errors.New("screenshot: feature disabled")
	ErrInvalidViewport    = errors.New("screenshot: invalid viewport")
	ErrScreenshotBusy     = errors.New("screenshot: capacity full")
	ErrScreenshotTimeout  = errors.New("screenshot: timeout")
	ErrScreenshotInternal = errors.New("screenshot: internal error")
)

// TemplateScreenshotStore is the subset of store methods the service needs.
type TemplateScreenshotStore interface {
	GetTemplateByID(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error)
	GetVersionByID(ctx context.Context, versionID uuid.UUID) (*domain.TemplateVersion, error)
	GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error)
}

// ScreenshotInput is the request shape for the service layer.
type ScreenshotInput struct {
	TemplateID uuid.UUID
	VersionID  *uuid.UUID
	Locale     string
	Viewport   string // "desktop" | "mobile"
}

// TemplateScreenshotService renders a stored template version to PNG.
type TemplateScreenshotService struct {
	store    TemplateScreenshotStore
	compiler port.TemplateCompiler
	capturer port.ScreenshotCapture
	cfg      config.ScreenshotConfig
}

// NewTemplateScreenshotService wires the service.
func NewTemplateScreenshotService(
	store TemplateScreenshotStore,
	compiler port.TemplateCompiler,
	capturer port.ScreenshotCapture,
	cfg config.ScreenshotConfig,
) *TemplateScreenshotService {
	return &TemplateScreenshotService{
		store:    store,
		compiler: compiler,
		capturer: capturer,
		cfg:      cfg,
	}
}

// Capture orchestrates resolution → compile → screenshot.
func (s *TemplateScreenshotService) Capture(ctx context.Context, in ScreenshotInput) ([]byte, error) {
	if !s.cfg.Enabled {
		return nil, ErrScreenshotDisabled
	}

	vp, err := s.viewportFor(in.Viewport)
	if err != nil {
		return nil, err
	}

	tpl, err := s.store.GetTemplateByID(ctx, in.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, domain.ErrTemplateNotFound
	}
	if tpl.IsDisabled {
		return nil, domain.ErrTemplateDisabled
	}

	var ver *domain.TemplateVersion
	if in.VersionID != nil {
		ver, err = s.store.GetVersionByID(ctx, *in.VersionID)
		if err != nil {
			return nil, err
		}
		if ver == nil {
			return nil, domain.ErrTemplateNotFound
		}
	} else {
		ver, err = s.store.GetPublishedVersion(ctx, in.TemplateID)
		if err != nil {
			return nil, err
		}
		if ver == nil {
			return nil, domain.ErrNoPublishedVersion
		}
	}

	mjml := ver.BodyMJML
	if in.Locale != "" {
		loc, err := s.store.GetLocale(ctx, ver.ID, in.Locale)
		if err != nil {
			return nil, err
		}
		if loc == nil {
			return nil, domain.ErrTemplateNotFound
		}
		if loc.BodyMJML != nil {
			mjml = *loc.BodyMJML
		}
	}

	html, err := s.compiler.Compile(ctx, mjml)
	if err != nil {
		return nil, err
	}

	png, err := s.capturer.Capture(ctx, html, vp, s.cfg.MaxHeightPx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, ErrScreenshotTimeout
		}
		return nil, errors.Join(ErrScreenshotInternal, err)
	}
	return png, nil
}

func (s *TemplateScreenshotService) viewportFor(name string) (port.Viewport, error) {
	switch name {
	case "desktop":
		return port.Viewport{Name: "desktop", WidthPx: s.cfg.DesktopWidthPx, DeviceScale: 1.0, MobileEmul: false}, nil
	case "mobile":
		return port.Viewport{Name: "mobile", WidthPx: s.cfg.MobileWidthPx, DeviceScale: 2.0, MobileEmul: true}, nil
	default:
		return port.Viewport{}, ErrInvalidViewport
	}
}

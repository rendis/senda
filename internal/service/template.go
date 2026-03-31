package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// TemplateService handles template, version, and locale business logic.
// It requires the full port.TemplateStore because it uses methods across all sub-interfaces:
//   - Core TemplateStore: CreateTemplate, ListByType, SetDisabled
//   - TemplateVersionStore: CreateVersion, GetVersionByID, UpdateVersion, Publish, ListVersions
//   - LocaleStore: SetLocale, GetLocale, ListLocales, DeleteLocale
type TemplateService struct {
	store    port.TemplateStore
	compiler port.TemplateCompiler
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(store port.TemplateStore, compiler port.TemplateCompiler) *TemplateService {
	return &TemplateService{store: store, compiler: compiler}
}

// CreateTemplate creates a new template for a given template type and workspace scope.
func (s *TemplateService) CreateTemplate(ctx context.Context, templateTypeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error) {
	now := time.Now().UTC()
	tpl := &domain.Template{
		ID:             uuid.Must(uuid.NewV7()),
		TemplateTypeID: templateTypeID,
		WorkspaceID:    wsID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.store.CreateTemplate(ctx, tpl); err != nil {
		return nil, err
	}

	return tpl, nil
}

// ListByType returns templates for a given template type and scope.
func (s *TemplateService) ListByType(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID, opts port.ListOptions) ([]*domain.Template, string, error) {
	return s.store.ListByType(ctx, typeID, wsID, opts)
}

// DisableTemplate activates the template kill switch for a scope.
func (s *TemplateService) DisableTemplate(ctx context.Context, templateID uuid.UUID, wsID *uuid.UUID) error {
	return s.store.SetDisabled(ctx, templateID, wsID, true)
}

// EnableTemplate deactivates the template kill switch for a scope.
func (s *TemplateService) EnableTemplate(ctx context.Context, templateID uuid.UUID, wsID *uuid.UUID) error {
	return s.store.SetDisabled(ctx, templateID, wsID, false)
}

// CreateVersion creates a new draft version for a template.
// Version assignment is delegated to the store to keep it transactional.
func (s *TemplateService) CreateVersion(ctx context.Context, templateID uuid.UUID, subject, previewText, fromName string, replyTo *string, bodyMJML, defaultLocale string, editorData map[string]any, createdBy *uuid.UUID) (*domain.TemplateVersion, error) {
	now := time.Now().UTC()
	ver := &domain.TemplateVersion{
		ID:            uuid.Must(uuid.NewV7()),
		TemplateID:    templateID,
		Status:        domain.VersionStatusDraft,
		Subject:       subject,
		PreviewText:   previewText,
		FromName:      fromName,
		ReplyTo:       replyTo,
		BodyMJML:      bodyMJML,
		DefaultLocale: defaultLocale,
		EditorData:    editorData,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.store.CreateVersion(ctx, ver); err != nil {
		return nil, err
	}

	return ver, nil
}

// GetVersionByID retrieves a single template version by its ID.
func (s *TemplateService) GetVersionByID(ctx context.Context, versionID uuid.UUID) (*domain.TemplateVersion, error) {
	return s.store.GetVersionByID(ctx, versionID)
}

// UpdateVersion updates a draft template version.
func (s *TemplateService) UpdateVersion(ctx context.Context, ver *domain.TemplateVersion) error {
	return s.store.UpdateVersion(ctx, ver)
}

// PublishVersion promotes a draft version to published, archiving any previously published version.
// The store's Publish method handles the atomic publish + archive.
func (s *TemplateService) PublishVersion(ctx context.Context, versionID uuid.UUID) error {
	return s.store.Publish(ctx, versionID)
}

// ListVersions returns all versions for a template.
func (s *TemplateService) ListVersions(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error) {
	return s.store.ListVersions(ctx, templateID)
}

// SetLocale creates or updates a locale override for a template version.
func (s *TemplateService) SetLocale(ctx context.Context, versionID uuid.UUID, locale string, subject, previewText, fromName, bodyMJML *string, editorData map[string]any) (*domain.TemplateVersionLocale, error) {
	now := time.Now().UTC()
	loc := &domain.TemplateVersionLocale{
		ID:                uuid.Must(uuid.NewV7()),
		TemplateVersionID: versionID,
		Locale:            locale,
		Subject:           subject,
		PreviewText:       previewText,
		FromName:          fromName,
		BodyMJML:          bodyMJML,
		EditorData:        editorData,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.store.SetLocale(ctx, loc); err != nil {
		return nil, err
	}

	return loc, nil
}

// GetLocale retrieves a locale override for a template version.
func (s *TemplateService) GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
	return s.store.GetLocale(ctx, versionID, locale)
}

// ListLocales returns all locale overrides for a template version.
func (s *TemplateService) ListLocales(ctx context.Context, versionID uuid.UUID) ([]*domain.TemplateVersionLocale, error) {
	return s.store.ListLocales(ctx, versionID)
}

// DeleteLocale removes a locale override for a template version.
func (s *TemplateService) DeleteLocale(ctx context.Context, versionID uuid.UUID, locale string) error {
	return s.store.DeleteLocale(ctx, versionID, locale)
}

// DeleteTemplate soft-deletes a template if it has no published version.
func (s *TemplateService) DeleteTemplate(ctx context.Context, templateID uuid.UUID) error {
	_, err := s.store.GetPublishedVersion(ctx, templateID)
	if err == nil {
		return domain.ErrHasPublishedVersion
	}
	return s.store.SoftDeleteTemplate(ctx, templateID)
}

// DeleteVersion hard-deletes a version if it's in draft status.
func (s *TemplateService) DeleteVersion(ctx context.Context, versionID uuid.UUID) error {
	ver, err := s.store.GetVersionByID(ctx, versionID)
	if err != nil {
		return err
	}
	if ver.Status != domain.VersionStatusDraft {
		return domain.ErrVersionNotDraft
	}
	return s.store.DeleteDraftVersion(ctx, versionID)
}

// PreviewMJML compiles MJML into HTML for preview.
func (s *TemplateService) PreviewMJML(ctx context.Context, mjml string) (string, error) {
	return s.compiler.Compile(ctx, mjml)
}

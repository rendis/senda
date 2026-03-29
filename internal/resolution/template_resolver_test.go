package resolution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
)

// --- Mock TemplateStore ---

type mockTemplateStore struct {
	getTypeBySlug       func(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error)
	resolveTemplate     func(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error)
	getPublishedVersion func(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error)
	getLocale           func(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error)
}

func (m *mockTemplateStore) CreateType(_ context.Context, _ *domain.TemplateType) error { return nil }
func (m *mockTemplateStore) GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error) {
	return m.getTypeBySlug(ctx, slug, chain)
}
func (m *mockTemplateStore) FindTypeBySlugInScope(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
	return nil, nil
}
func (m *mockTemplateStore) CreateTemplate(_ context.Context, _ *domain.Template) error { return nil }
func (m *mockTemplateStore) GetByTypeAndScope(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (*domain.Template, error) {
	return nil, nil
}
func (m *mockTemplateStore) ResolveTemplate(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error) {
	return m.resolveTemplate(ctx, typeID, chain)
}
func (m *mockTemplateStore) SetDisabled(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ bool) error {
	return nil
}
func (m *mockTemplateStore) CreateVersion(_ context.Context, _ *domain.TemplateVersion) error {
	return nil
}
func (m *mockTemplateStore) GetVersionByID(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
	return nil, nil
}
func (m *mockTemplateStore) GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
	return m.getPublishedVersion(ctx, templateID)
}
func (m *mockTemplateStore) UpdateVersion(_ context.Context, _ *domain.TemplateVersion) error {
	return nil
}
func (m *mockTemplateStore) Publish(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockTemplateStore) ListVersions(_ context.Context, _ uuid.UUID) ([]*domain.TemplateVersion, error) {
	return nil, nil
}
func (m *mockTemplateStore) SetLocale(_ context.Context, _ *domain.TemplateVersionLocale) error {
	return nil
}
func (m *mockTemplateStore) GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
	return m.getLocale(ctx, versionID, locale)
}
func (m *mockTemplateStore) ListLocales(_ context.Context, _ uuid.UUID) ([]*domain.TemplateVersionLocale, error) {
	return nil, nil
}
func (m *mockTemplateStore) DeleteLocale(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockTemplateStore) ListByType(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ port.ListOptions) ([]*domain.Template, string, error) {
	return nil, "", nil
}
func (m *mockTemplateStore) ListTypes(_ context.Context, _ *uuid.UUID, _ port.ListOptions) ([]*domain.TemplateType, string, error) {
	return nil, "", nil
}

// --- Tests ---

func TestTemplateResolver_FullSuccess_NoLocale(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tplID := uuid.New()
	verID := uuid.New()

	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}
	tpl := &domain.Template{ID: tplID, TemplateTypeID: typeID, IsDisabled: false}
	ver := &domain.TemplateVersion{ID: verID, TemplateID: tplID, Status: domain.VersionStatusPublished, DefaultLocale: "en"}

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return tpl, nil
		},
		getPublishedVersion: func(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
			return ver, nil
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	result, err := resolver.Resolve(context.Background(), wsID, "welcome-email", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Template.ID != tplID {
		t.Errorf("Template.ID = %v, want %v", result.Template.ID, tplID)
	}
	if result.Version.ID != verID {
		t.Errorf("Version.ID = %v, want %v", result.Version.ID, verID)
	}
	if result.TemplateType.ID != typeID {
		t.Errorf("TemplateType.ID = %v, want %v", result.TemplateType.ID, typeID)
	}
	if result.Locale != nil {
		t.Errorf("Locale = %v, want nil", result.Locale)
	}
}

func TestTemplateResolver_TypeNotFound(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return nil, errors.New("not found")
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	_, err := resolver.Resolve(context.Background(), wsID, "nonexistent", nil)
	if !errors.Is(err, domain.ErrTemplateTypeNotFound) {
		t.Errorf("expected ErrTemplateTypeNotFound, got %v", err)
	}
}

func TestTemplateResolver_TemplateNotFound(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return nil, errors.New("not found")
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	_, err := resolver.Resolve(context.Background(), wsID, "welcome-email", nil)
	if !errors.Is(err, domain.ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateResolver_TemplateDisabled(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tplID := uuid.New()
	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}
	tpl := &domain.Template{ID: tplID, TemplateTypeID: typeID, IsDisabled: true}

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return tpl, nil
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	_, err := resolver.Resolve(context.Background(), wsID, "welcome-email", nil)
	if !errors.Is(err, domain.ErrTemplateDisabled) {
		t.Errorf("expected ErrTemplateDisabled, got %v", err)
	}
}

func TestTemplateResolver_NoPublishedVersion(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tplID := uuid.New()
	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}
	tpl := &domain.Template{ID: tplID, TemplateTypeID: typeID, IsDisabled: false}

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return tpl, nil
		},
		getPublishedVersion: func(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
			return nil, errors.New("no version")
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	_, err := resolver.Resolve(context.Background(), wsID, "welcome-email", nil)
	if !errors.Is(err, domain.ErrNoPublishedVersion) {
		t.Errorf("expected ErrNoPublishedVersion, got %v", err)
	}
}

func TestTemplateResolver_LocaleExactMatch(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tplID := uuid.New()
	verID := uuid.New()
	localeID := uuid.New()

	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}
	tpl := &domain.Template{ID: tplID, TemplateTypeID: typeID, IsDisabled: false}
	ver := &domain.TemplateVersion{ID: verID, TemplateID: tplID, Status: domain.VersionStatusPublished, DefaultLocale: "en"}
	loc := &domain.TemplateVersionLocale{ID: localeID, TemplateVersionID: verID, Locale: "es"}

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return tpl, nil
		},
		getPublishedVersion: func(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
			return ver, nil
		},
		getLocale: func(_ context.Context, _ uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
			if locale == "es" {
				return loc, nil
			}
			return nil, errors.New("not found")
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	locale := "es"
	result, err := resolver.Resolve(context.Background(), wsID, "welcome-email", &locale)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Locale == nil {
		t.Fatal("Locale is nil, want non-nil")
	}
	if result.Locale.ID != localeID {
		t.Errorf("Locale.ID = %v, want %v", result.Locale.ID, localeID)
	}
}

func TestTemplateResolver_LocalePrefixFallback(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tplID := uuid.New()
	verID := uuid.New()
	localeID := uuid.New()

	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}
	tpl := &domain.Template{ID: tplID, TemplateTypeID: typeID, IsDisabled: false}
	ver := &domain.TemplateVersion{ID: verID, TemplateID: tplID, Status: domain.VersionStatusPublished, DefaultLocale: "en"}
	loc := &domain.TemplateVersionLocale{ID: localeID, TemplateVersionID: verID, Locale: "es"}

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return tpl, nil
		},
		getPublishedVersion: func(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
			return ver, nil
		},
		getLocale: func(_ context.Context, _ uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
			if locale == "es" {
				return loc, nil
			}
			return nil, errors.New("not found")
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	locale := "es-CO"
	result, err := resolver.Resolve(context.Background(), wsID, "welcome-email", &locale)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Locale == nil {
		t.Fatal("Locale is nil, want non-nil (prefix fallback to 'es')")
	}
	if result.Locale.Locale != "es" {
		t.Errorf("Locale.Locale = %q, want %q", result.Locale.Locale, "es")
	}
}

func TestTemplateResolver_LocaleTotalMiss(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tplID := uuid.New()
	verID := uuid.New()

	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}
	tpl := &domain.Template{ID: tplID, TemplateTypeID: typeID, IsDisabled: false}
	ver := &domain.TemplateVersion{ID: verID, TemplateID: tplID, Status: domain.VersionStatusPublished, DefaultLocale: "en"}

	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return tpl, nil
		},
		getPublishedVersion: func(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
			return ver, nil
		},
		getLocale: func(_ context.Context, _ uuid.UUID, _ string) (*domain.TemplateVersionLocale, error) {
			return nil, errors.New("not found")
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	locale := "zh"
	result, err := resolver.Resolve(context.Background(), wsID, "welcome-email", &locale)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Locale != nil {
		t.Errorf("Locale = %v, want nil (total miss falls back to default)", result.Locale)
	}
}

func TestTemplateResolver_LocaleMatchesDefault_SkipLookup(t *testing.T) {
	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          tenantID,
		Scopes: []uuid.NullUUID{
			{UUID: wsID, Valid: true},
			{UUID: sysID, Valid: true},
			{Valid: false},
		},
	}
	cr := newTestChainResolver(chain, nil)

	typeID := uuid.New()
	tplID := uuid.New()
	verID := uuid.New()

	tt := &domain.TemplateType{ID: typeID, Slug: "welcome-email"}
	tpl := &domain.Template{ID: tplID, TemplateTypeID: typeID, IsDisabled: false}
	ver := &domain.TemplateVersion{ID: verID, TemplateID: tplID, Status: domain.VersionStatusPublished, DefaultLocale: "en"}

	localeLookupCalled := false
	store := &mockTemplateStore{
		getTypeBySlug: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return tt, nil
		},
		resolveTemplate: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			return tpl, nil
		},
		getPublishedVersion: func(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
			return ver, nil
		},
		getLocale: func(_ context.Context, _ uuid.UUID, _ string) (*domain.TemplateVersionLocale, error) {
			localeLookupCalled = true
			return nil, errors.New("should not be called")
		},
	}

	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)
	locale := "en" // same as DefaultLocale
	result, err := resolver.Resolve(context.Background(), wsID, "welcome-email", &locale)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if localeLookupCalled {
		t.Error("GetLocale was called even though locale matches default")
	}
	if result.Locale != nil {
		t.Errorf("Locale = %v, want nil (matches default)", result.Locale)
	}
}

func TestTemplateResolver_ChainResolverError(t *testing.T) {
	chainErr := errors.New("chain error")
	cr := newErrorChainResolver(chainErr)

	store := &mockTemplateStore{}
	resolver := resolution.NewTemplateResolver(store, newMockCache(), cr)

	_, err := resolver.Resolve(context.Background(), uuid.New(), "any-slug", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

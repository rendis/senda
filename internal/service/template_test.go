package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// --- Mock TemplateStore ---

type mockTemplateStore struct {
	createTypeFn            func(ctx context.Context, tt *domain.TemplateType) error
	getTypeBySlugFn         func(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error)
	findTypeBySlugInScopeFn func(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error)
	createTemplateFn        func(ctx context.Context, tpl *domain.Template) error
	getByTypeAndScopeFn     func(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error)
	resolveTemplateFn       func(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error)
	createVersionFn         func(ctx context.Context, ver *domain.TemplateVersion) error
	getPublishedVersionFn   func(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error)
	publishFn               func(ctx context.Context, versionID uuid.UUID) error
	setDisabledFn           func(ctx context.Context, templateID uuid.UUID, wsID *uuid.UUID, disabled bool) error
	listVersionsFn          func(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error)
	setLocaleFn             func(ctx context.Context, locale *domain.TemplateVersionLocale) error
	getLocaleFn             func(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error)
}

func (m *mockTemplateStore) CreateType(ctx context.Context, tt *domain.TemplateType) error {
	if m.createTypeFn != nil {
		return m.createTypeFn(ctx, tt)
	}
	return nil
}
func (m *mockTemplateStore) GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error) {
	if m.getTypeBySlugFn != nil {
		return m.getTypeBySlugFn(ctx, slug, chain)
	}
	return nil, nil
}
func (m *mockTemplateStore) FindTypeBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error) {
	if m.findTypeBySlugInScopeFn != nil {
		return m.findTypeBySlugInScopeFn(ctx, slug, wsID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStore) CreateTemplate(ctx context.Context, tpl *domain.Template) error {
	if m.createTemplateFn != nil {
		return m.createTemplateFn(ctx, tpl)
	}
	return nil
}
func (m *mockTemplateStore) GetByTypeAndScope(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error) {
	if m.getByTypeAndScopeFn != nil {
		return m.getByTypeAndScopeFn(ctx, typeID, wsID)
	}
	return nil, nil
}
func (m *mockTemplateStore) ResolveTemplate(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error) {
	if m.resolveTemplateFn != nil {
		return m.resolveTemplateFn(ctx, typeID, chain)
	}
	return nil, nil
}
func (m *mockTemplateStore) SetDisabled(ctx context.Context, templateID uuid.UUID, wsID *uuid.UUID, disabled bool) error {
	if m.setDisabledFn != nil {
		return m.setDisabledFn(ctx, templateID, wsID, disabled)
	}
	return nil
}
func (m *mockTemplateStore) CreateVersion(ctx context.Context, ver *domain.TemplateVersion) error {
	if m.createVersionFn != nil {
		return m.createVersionFn(ctx, ver)
	}
	return nil
}
func (m *mockTemplateStore) GetVersionByID(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
	return nil, nil
}
func (m *mockTemplateStore) GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
	if m.getPublishedVersionFn != nil {
		return m.getPublishedVersionFn(ctx, templateID)
	}
	return nil, nil
}
func (m *mockTemplateStore) UpdateVersion(_ context.Context, _ *domain.TemplateVersion) error {
	return nil
}
func (m *mockTemplateStore) Publish(ctx context.Context, versionID uuid.UUID) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, versionID)
	}
	return nil
}
func (m *mockTemplateStore) GetLatestVersion(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStore) SoftDeleteTemplate(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockTemplateStore) DeleteDraftVersion(_ context.Context, _ uuid.UUID) error  { return nil }
func (m *mockTemplateStore) ListVersions(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error) {
	if m.listVersionsFn != nil {
		return m.listVersionsFn(ctx, templateID)
	}
	return nil, nil
}
func (m *mockTemplateStore) SetLocale(ctx context.Context, locale *domain.TemplateVersionLocale) error {
	if m.setLocaleFn != nil {
		return m.setLocaleFn(ctx, locale)
	}
	return nil
}
func (m *mockTemplateStore) GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
	if m.getLocaleFn != nil {
		return m.getLocaleFn(ctx, versionID, locale)
	}
	return nil, nil
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
func (m *mockTemplateStore) UpdateType(_ context.Context, _ *domain.TemplateType) error { return nil }
func (m *mockTemplateStore) SoftDeleteType(_ context.Context, _ uuid.UUID) error              { return nil }
func (m *mockTemplateStore) GetTemplateByID(_ context.Context, _ uuid.UUID) (*domain.Template, error) {
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStore) GetTypeByID(_ context.Context, _ uuid.UUID) (*domain.TemplateType, error) {
	return nil, domain.ErrNotFound
}

// --- Mock TemplateCompiler ---

type mockTemplateCompiler struct {
	compileFn func(ctx context.Context, mjml string) (string, error)
}

func (m *mockTemplateCompiler) Compile(ctx context.Context, mjml string) (string, error) {
	if m.compileFn != nil {
		return m.compileFn(ctx, mjml)
	}
	return "<html><body>compiled</body></html>", nil
}

// --- TemplateService Tests ---

func TestTemplateService_CreateTemplate_Success(t *testing.T) {
	var created *domain.Template
	store := &mockTemplateStore{
		createTemplateFn: func(_ context.Context, tpl *domain.Template) error {
			created = tpl
			return nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	typeID := uuid.Must(uuid.NewV7())
	wsID := uuid.Must(uuid.NewV7())
	tpl, err := svc.CreateTemplate(context.Background(), typeID, &wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.TemplateTypeID != typeID {
		t.Fatalf("expected template type ID %s, got %s", typeID, tpl.TemplateTypeID)
	}
	if tpl.WorkspaceID == nil || *tpl.WorkspaceID != wsID {
		t.Fatalf("expected workspace ID %s", wsID)
	}
	if tpl.IsDisabled {
		t.Fatal("expected is_disabled=false by default")
	}
	if created == nil {
		t.Fatal("expected template to be persisted")
	}
}

func TestTemplateService_CreateTemplate_StoreError(t *testing.T) {
	store := &mockTemplateStore{
		createTemplateFn: func(_ context.Context, _ *domain.Template) error {
			return domain.ErrConflict
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	_, err := svc.CreateTemplate(context.Background(), uuid.Must(uuid.NewV7()), nil)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestTemplateService_CreateVersion_Success(t *testing.T) {
	var created *domain.TemplateVersion
	templateID := uuid.Must(uuid.NewV7())

	store := &mockTemplateStore{
		createVersionFn: func(_ context.Context, ver *domain.TemplateVersion) error {
			ver.VersionNumber = 2
			created = ver
			return nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	replyTo := "reply@example.com"
	ver, err := svc.CreateVersion(context.Background(), templateID,
		"Welcome {{name}}", "Preview text", "Acme",
		&replyTo, "<mjml><mj-body></mj-body></mjml>", "en",
		map[string]any{"editor": "v1"}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver.Status != domain.VersionStatusDraft {
		t.Fatalf("expected status 'draft', got %q", ver.Status)
	}
	if ver.VersionNumber != 2 {
		t.Fatalf("expected version number 2, got %d", ver.VersionNumber)
	}
	if ver.TemplateID != templateID {
		t.Fatalf("expected template ID %s, got %s", templateID, ver.TemplateID)
	}
	if created == nil {
		t.Fatal("expected version to be persisted")
	}
}

func TestTemplateService_CreateVersion_FirstVersion(t *testing.T) {
	store := &mockTemplateStore{
		createVersionFn: func(_ context.Context, ver *domain.TemplateVersion) error {
			ver.VersionNumber = 1
			return nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	ver, err := svc.CreateVersion(context.Background(), uuid.Must(uuid.NewV7()),
		"Subject", "Preview", "From",
		nil, "<mjml></mjml>", "en", nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver.VersionNumber != 1 {
		t.Fatalf("expected version number 1, got %d", ver.VersionNumber)
	}
}

func TestTemplateService_CreateVersion_StoreError(t *testing.T) {
	store := &mockTemplateStore{
		createVersionFn: func(_ context.Context, _ *domain.TemplateVersion) error {
			return errors.New("db error")
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	_, err := svc.CreateVersion(context.Background(), uuid.Must(uuid.NewV7()),
		"Subject", "Preview", "From",
		nil, "<mjml></mjml>", "en", nil, nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTemplateService_DisableTemplate_Success(t *testing.T) {
	templateID := uuid.Must(uuid.NewV7())
	wsID := uuid.Must(uuid.NewV7())

	var (
		gotTemplateID uuid.UUID
		gotWorkspace  *uuid.UUID
		gotDisabled   bool
	)

	store := &mockTemplateStore{
		setDisabledFn: func(_ context.Context, id uuid.UUID, scope *uuid.UUID, disabled bool) error {
			gotTemplateID = id
			gotWorkspace = scope
			gotDisabled = disabled
			return nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})
	if err := svc.DisableTemplate(context.Background(), templateID, &wsID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotTemplateID != templateID {
		t.Fatalf("expected template ID %s, got %s", templateID, gotTemplateID)
	}
	if gotWorkspace == nil || *gotWorkspace != wsID {
		t.Fatalf("expected workspace ID %s, got %v", wsID, gotWorkspace)
	}
	if !gotDisabled {
		t.Fatal("expected disabled=true")
	}
}

func TestTemplateService_EnableTemplate_Success(t *testing.T) {
	templateID := uuid.Must(uuid.NewV7())

	var gotDisabled bool
	store := &mockTemplateStore{
		setDisabledFn: func(_ context.Context, _ uuid.UUID, _ *uuid.UUID, disabled bool) error {
			gotDisabled = disabled
			return nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})
	if err := svc.EnableTemplate(context.Background(), templateID, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotDisabled {
		t.Fatal("expected disabled=false")
	}
}

func TestTemplateService_PublishVersion_Success(t *testing.T) {
	var publishedID uuid.UUID
	store := &mockTemplateStore{
		publishFn: func(_ context.Context, id uuid.UUID) error {
			publishedID = id
			return nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	versionID := uuid.Must(uuid.NewV7())
	err := svc.PublishVersion(context.Background(), versionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if publishedID != versionID {
		t.Fatalf("expected published version ID %s, got %s", versionID, publishedID)
	}
}

func TestTemplateService_PublishVersion_Error(t *testing.T) {
	store := &mockTemplateStore{
		publishFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrNotFound
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	err := svc.PublishVersion(context.Background(), uuid.Must(uuid.NewV7()))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTemplateService_ListVersions_Success(t *testing.T) {
	templateID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	expected := []*domain.TemplateVersion{
		{ID: uuid.Must(uuid.NewV7()), TemplateID: templateID, VersionNumber: 1, Status: domain.VersionStatusPublished, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.Must(uuid.NewV7()), TemplateID: templateID, VersionNumber: 2, Status: domain.VersionStatusDraft, CreatedAt: now, UpdatedAt: now},
	}

	store := &mockTemplateStore{
		listVersionsFn: func(_ context.Context, id uuid.UUID) ([]*domain.TemplateVersion, error) {
			if id == templateID {
				return expected, nil
			}
			return nil, nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	versions, err := svc.ListVersions(context.Background(), templateID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
}

func TestTemplateService_SetLocale_Success(t *testing.T) {
	var set *domain.TemplateVersionLocale
	store := &mockTemplateStore{
		setLocaleFn: func(_ context.Context, loc *domain.TemplateVersionLocale) error {
			set = loc
			return nil
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	versionID := uuid.Must(uuid.NewV7())
	subject := "Bienvenido"
	loc, err := svc.SetLocale(context.Background(), versionID, "es", &subject, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loc.Locale != "es" {
		t.Fatalf("expected locale 'es', got %q", loc.Locale)
	}
	if loc.Subject == nil || *loc.Subject != "Bienvenido" {
		t.Fatal("expected subject 'Bienvenido'")
	}
	if loc.TemplateVersionID != versionID {
		t.Fatalf("expected version ID %s", versionID)
	}
	if set == nil {
		t.Fatal("expected locale to be persisted")
	}
}

func TestTemplateService_SetLocale_StoreError(t *testing.T) {
	store := &mockTemplateStore{
		setLocaleFn: func(_ context.Context, _ *domain.TemplateVersionLocale) error {
			return domain.ErrConflict
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	_, err := svc.SetLocale(context.Background(), uuid.Must(uuid.NewV7()), "fr", nil, nil, nil, nil, nil)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestTemplateService_GetLocale_Success(t *testing.T) {
	versionID := uuid.Must(uuid.NewV7())
	subject := "Bonjour"
	expected := &domain.TemplateVersionLocale{
		ID:                uuid.Must(uuid.NewV7()),
		TemplateVersionID: versionID,
		Locale:            "fr",
		Subject:           &subject,
	}

	store := &mockTemplateStore{
		getLocaleFn: func(_ context.Context, vid uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
			if vid == versionID && locale == "fr" {
				return expected, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	loc, err := svc.GetLocale(context.Background(), versionID, "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loc.Locale != "fr" {
		t.Fatalf("expected locale 'fr', got %q", loc.Locale)
	}
}

func TestTemplateService_GetLocale_NotFound(t *testing.T) {
	store := &mockTemplateStore{
		getLocaleFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.TemplateVersionLocale, error) {
			return nil, domain.ErrNotFound
		},
	}

	svc := service.NewTemplateService(store, &mockTemplateCompiler{})

	_, err := svc.GetLocale(context.Background(), uuid.Must(uuid.NewV7()), "zh")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTemplateService_PreviewMJML_Success(t *testing.T) {
	compiler := &mockTemplateCompiler{
		compileFn: func(_ context.Context, mjml string) (string, error) {
			return "<html><body>" + mjml + "</body></html>", nil
		},
	}

	svc := service.NewTemplateService(&mockTemplateStore{}, compiler)

	html, err := svc.PreviewMJML(context.Background(), "<mjml><mj-body><mj-section></mj-section></mj-body></mjml>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html == "" {
		t.Fatal("expected non-empty HTML")
	}
}

func TestTemplateService_PreviewMJML_CompilerError(t *testing.T) {
	compiler := &mockTemplateCompiler{
		compileFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("invalid mjml")
		},
	}

	svc := service.NewTemplateService(&mockTemplateStore{}, compiler)

	_, err := svc.PreviewMJML(context.Background(), "bad mjml")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid mjml" {
		t.Fatalf("expected 'invalid mjml', got %q", err.Error())
	}
}

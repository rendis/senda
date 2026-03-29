package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/resolution"
	"github.com/senda-app/senda/internal/service"
)

// --- Send-specific mocks (suffixed to avoid collisions with other test files) ---

type mockTenantStoreSend struct {
	getByCodeFn func(ctx context.Context, code string) (*domain.Tenant, error)
}

func (m *mockTenantStoreSend) Create(_ context.Context, _ *domain.Tenant) error { return nil }
func (m *mockTenantStoreSend) GetByID(_ context.Context, _ uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (m *mockTenantStoreSend) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	if m.getByCodeFn != nil {
		return m.getByCodeFn(ctx, code)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTenantStoreSend) List(_ context.Context, _ port.ListOptions) ([]*domain.Tenant, string, error) {
	return nil, "", nil
}
func (m *mockTenantStoreSend) Update(_ context.Context, _ *domain.Tenant) error { return nil }
func (m *mockTenantStoreSend) SoftDelete(_ context.Context, _ uuid.UUID) error  { return nil }
func (m *mockTenantStoreSend) Purge(_ context.Context, _ uuid.UUID) error       { return nil }

type mockWorkspaceStoreSend struct {
	getByTenantAndCodeFn func(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error)
	getByIDFn            func(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	getSystemWorkspaceFn func(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error)
}

func (m *mockWorkspaceStoreSend) Create(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStoreSend) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStoreSend) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
	if m.getByTenantAndCodeFn != nil {
		return m.getByTenantAndCodeFn(ctx, tenantID, code)
	}
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStoreSend) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
	if m.getSystemWorkspaceFn != nil {
		return m.getSystemWorkspaceFn(ctx, tenantID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStoreSend) ListByTenant(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
	return nil, "", nil
}
func (m *mockWorkspaceStoreSend) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStoreSend) SoftDelete(_ context.Context, _ uuid.UUID) error     { return nil }

type mockEmailStoreSend struct {
	createFn   func(ctx context.Context, email *domain.Email) error
	addEventFn func(ctx context.Context, event *domain.EmailEvent) error
	emails     []*domain.Email
	events     []*domain.EmailEvent
}

func (m *mockEmailStoreSend) Create(ctx context.Context, email *domain.Email) error {
	m.emails = append(m.emails, email)
	if m.createFn != nil {
		return m.createFn(ctx, email)
	}
	return nil
}
func (m *mockEmailStoreSend) CreateTx(_ context.Context, _ pgx.Tx, _ *domain.Email) error {
	return nil
}
func (m *mockEmailStoreSend) GetByTrackingID(_ context.Context, _ string) (*domain.Email, error) {
	return nil, nil
}
func (m *mockEmailStoreSend) GetByProviderMessageID(_ context.Context, _ string) (*domain.Email, error) {
	return nil, nil
}
func (m *mockEmailStoreSend) UpdateStatus(_ context.Context, _ uuid.UUID, _, _ domain.EmailStatus) error {
	return nil
}
func (m *mockEmailStoreSend) UpdateRetry(_ context.Context, _ uuid.UUID, _ int, _ *time.Time) error {
	return nil
}
func (m *mockEmailStoreSend) SetProviderMessageID(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockEmailStoreSend) AddEvent(ctx context.Context, event *domain.EmailEvent) error {
	m.events = append(m.events, event)
	if m.addEventFn != nil {
		return m.addEventFn(ctx, event)
	}
	return nil
}
func (m *mockEmailStoreSend) GetEvents(_ context.Context, _ uuid.UUID) ([]*domain.EmailEvent, error) {
	return nil, nil
}
func (m *mockEmailStoreSend) QueryByExternalID(_ context.Context, _ uuid.UUID, _ string, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}
func (m *mockEmailStoreSend) QueryByRecipient(_ context.Context, _ uuid.UUID, _ string, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}
func (m *mockEmailStoreSend) QueryByWorkspace(_ context.Context, _ uuid.UUID, _ port.EmailFilters, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}
func (m *mockEmailStoreSend) QueryByExternalIDGlobal(_ context.Context, _ string, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}

type mockSuppressionStoreSend struct {
	isSuppressedFn func(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error)
}

func (m *mockSuppressionStoreSend) AddGlobal(_ context.Context, _ *domain.SuppressionGlobal) error {
	return nil
}
func (m *mockSuppressionStoreSend) IsGloballySuppressed(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (m *mockSuppressionStoreSend) RemoveGlobal(_ context.Context, _ string, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockSuppressionStoreSend) AddWorkspace(_ context.Context, _ *domain.SuppressionWorkspace) error {
	return nil
}
func (m *mockSuppressionStoreSend) IsWorkspaceSuppressed(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return false, nil
}
func (m *mockSuppressionStoreSend) IsSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error) {
	if m.isSuppressedFn != nil {
		return m.isSuppressedFn(ctx, wsID, email)
	}
	return false, "", nil
}

type mockCacheSend struct {
	data map[string][]byte
}

func newMockCacheSend() *mockCacheSend {
	return &mockCacheSend{data: make(map[string][]byte)}
}

func (m *mockCacheSend) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, errors.New("cache miss")
	}
	return v, nil
}
func (m *mockCacheSend) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.data[key] = value
	return nil
}
func (m *mockCacheSend) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}
func (m *mockCacheSend) DeletePattern(_ context.Context, _ string) error {
	return nil
}

type mockTemplateStoreSend struct {
	getTypeBySlugFn         func(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error)
	findTypeBySlugInScopeFn func(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error)
	createTypeFn            func(ctx context.Context, tt *domain.TemplateType) error
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

func (m *mockTemplateStoreSend) CreateType(ctx context.Context, tt *domain.TemplateType) error {
	if m.createTypeFn != nil {
		return m.createTypeFn(ctx, tt)
	}
	return nil
}
func (m *mockTemplateStoreSend) GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error) {
	if m.getTypeBySlugFn != nil {
		return m.getTypeBySlugFn(ctx, slug, chain)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStoreSend) FindTypeBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error) {
	if m.findTypeBySlugInScopeFn != nil {
		return m.findTypeBySlugInScopeFn(ctx, slug, wsID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStoreSend) CreateTemplate(ctx context.Context, tpl *domain.Template) error {
	if m.createTemplateFn != nil {
		return m.createTemplateFn(ctx, tpl)
	}
	return nil
}
func (m *mockTemplateStoreSend) GetByTypeAndScope(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error) {
	if m.getByTypeAndScopeFn != nil {
		return m.getByTypeAndScopeFn(ctx, typeID, wsID)
	}
	return nil, nil
}
func (m *mockTemplateStoreSend) ListByType(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ port.ListOptions) ([]*domain.Template, string, error) {
	return nil, "", nil
}
func (m *mockTemplateStoreSend) ResolveTemplate(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error) {
	if m.resolveTemplateFn != nil {
		return m.resolveTemplateFn(ctx, typeID, chain)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStoreSend) SetDisabled(ctx context.Context, templateID uuid.UUID, wsID *uuid.UUID, disabled bool) error {
	if m.setDisabledFn != nil {
		return m.setDisabledFn(ctx, templateID, wsID, disabled)
	}
	return nil
}
func (m *mockTemplateStoreSend) CreateVersion(ctx context.Context, ver *domain.TemplateVersion) error {
	if m.createVersionFn != nil {
		return m.createVersionFn(ctx, ver)
	}
	return nil
}
func (m *mockTemplateStoreSend) GetVersionByID(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
	return nil, nil
}
func (m *mockTemplateStoreSend) GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
	if m.getPublishedVersionFn != nil {
		return m.getPublishedVersionFn(ctx, templateID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStoreSend) UpdateVersion(_ context.Context, _ *domain.TemplateVersion) error {
	return nil
}
func (m *mockTemplateStoreSend) Publish(ctx context.Context, versionID uuid.UUID) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, versionID)
	}
	return nil
}
func (m *mockTemplateStoreSend) ListVersions(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error) {
	if m.listVersionsFn != nil {
		return m.listVersionsFn(ctx, templateID)
	}
	return nil, nil
}
func (m *mockTemplateStoreSend) SetLocale(ctx context.Context, locale *domain.TemplateVersionLocale) error {
	if m.setLocaleFn != nil {
		return m.setLocaleFn(ctx, locale)
	}
	return nil
}
func (m *mockTemplateStoreSend) GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
	if m.getLocaleFn != nil {
		return m.getLocaleFn(ctx, versionID, locale)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStoreSend) ListLocales(_ context.Context, _ uuid.UUID) ([]*domain.TemplateVersionLocale, error) {
	return nil, nil
}
func (m *mockTemplateStoreSend) DeleteLocale(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockTemplateStoreSend) ListTypes(_ context.Context, _ *uuid.UUID, _ port.ListOptions) ([]*domain.TemplateType, string, error) {
	return nil, "", nil
}

type mockInjectorStoreSend struct {
	listDefinitionsInChainFn func(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)
	getFieldsByDefinitionFn  func(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)
	getValuesFn              func(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)
}

func (m *mockInjectorStoreSend) CreateDefinition(_ context.Context, _ *domain.InjectorDefinition) error {
	return nil
}
func (m *mockInjectorStoreSend) GetDefinitionByID(_ context.Context, _ uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStoreSend) FindDefinitionByName(_ context.Context, _ string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStoreSend) ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
	if m.listDefinitionsInChainFn != nil {
		return m.listDefinitionsInChainFn(ctx, chain)
	}
	return nil, nil
}
func (m *mockInjectorStoreSend) CreateField(_ context.Context, _ *domain.InjectorField) error {
	return nil
}
func (m *mockInjectorStoreSend) GetFieldsByDefinition(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
	if m.getFieldsByDefinitionFn != nil {
		return m.getFieldsByDefinitionFn(ctx, defID)
	}
	return nil, nil
}
func (m *mockInjectorStoreSend) SetValue(_ context.Context, _ *domain.InjectorValue) error {
	return nil
}
func (m *mockInjectorStoreSend) GetValues(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error) {
	if m.getValuesFn != nil {
		return m.getValuesFn(ctx, defID, chain)
	}
	return nil, nil
}
func (m *mockInjectorStoreSend) GetAllFieldsByDefinitions(_ context.Context, _ []uuid.UUID) (map[uuid.UUID][]*domain.InjectorField, error) {
	return nil, nil
}
func (m *mockInjectorStoreSend) GetAllValuesByDefinitions(_ context.Context, _ []uuid.UUID, _ []uuid.NullUUID) (map[uuid.UUID][]*domain.InjectorValue, error) {
	return nil, nil
}

type mockAdapterIdentityStoreSend struct {
	getDefaultFn    func(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error)
	listByAdapterFn func(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error)
}

func (m *mockAdapterIdentityStoreSend) Create(_ context.Context, _ *domain.AdapterIdentity) error {
	return nil
}
func (m *mockAdapterIdentityStoreSend) GetByID(_ context.Context, _ uuid.UUID) (*domain.AdapterIdentity, error) {
	return nil, nil
}
func (m *mockAdapterIdentityStoreSend) Update(_ context.Context, _ *domain.AdapterIdentity) error {
	return nil
}
func (m *mockAdapterIdentityStoreSend) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockAdapterIdentityStoreSend) ListByAdapter(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	if m.listByAdapterFn != nil {
		return m.listByAdapterFn(ctx, adapterID)
	}
	return nil, nil
}
func (m *mockAdapterIdentityStoreSend) GetDefault(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error) {
	if m.getDefaultFn != nil {
		return m.getDefaultFn(ctx, adapterID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockAdapterIdentityStoreSend) SetDefault(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (m *mockAdapterIdentityStoreSend) UpsertBatch(_ context.Context, _ uuid.UUID, _ []*domain.AdapterIdentity) error {
	return nil
}
func (m *mockAdapterIdentityStoreSend) DeleteStale(_ context.Context, _ uuid.UUID, _ []string) error {
	return nil
}

type mockAdapterStoreSend struct {
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Adapter, error)
}

func (m *mockAdapterStoreSend) Create(_ context.Context, _ *domain.Adapter) error { return nil }
func (m *mockAdapterStoreSend) GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockAdapterStoreSend) Update(_ context.Context, _ *domain.Adapter) error { return nil }
func (m *mockAdapterStoreSend) SoftDelete(_ context.Context, _ uuid.UUID) error   { return nil }
func (m *mockAdapterStoreSend) ListInChain(_ context.Context, _ []uuid.NullUUID) ([]*domain.Adapter, error) {
	return nil, nil
}
func (m *mockAdapterStoreSend) ListByWorkspace(_ context.Context, _ *uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	return nil, nil
}

type mockJobQueueSend struct {
	enqueueSendFn    func(ctx context.Context, job *port.SendJob) error
	enqueueWebhookFn func(ctx context.Context, job *port.WebhookJob) error
}

func (m *mockJobQueueSend) EnqueueSend(ctx context.Context, job *port.SendJob) error {
	if m.enqueueSendFn != nil {
		return m.enqueueSendFn(ctx, job)
	}
	return nil
}
func (m *mockJobQueueSend) EnqueueSendTx(_ context.Context, _ pgx.Tx, _ *port.SendJob) error {
	return nil
}
func (m *mockJobQueueSend) EnqueueWebhook(ctx context.Context, job *port.WebhookJob) error {
	if m.enqueueWebhookFn != nil {
		return m.enqueueWebhookFn(ctx, job)
	}
	return nil
}

// --- Helper to build a complete test fixture ---

type sendTestFixture struct {
	tenantID    uuid.UUID
	workspaceID uuid.UUID
	sysWSID     uuid.UUID
	templateID  uuid.UUID
	versionID   uuid.UUID
	typeID      uuid.UUID
	adapterID   uuid.UUID

	tenantStore   *mockTenantStoreSend
	wsStore       *mockWorkspaceStoreSend
	emailStore    *mockEmailStoreSend
	suppression   *mockSuppressionStoreSend
	jq            *mockJobQueueSend
	cache         *mockCacheSend
	templateStore *mockTemplateStoreSend
	injectorStore *mockInjectorStoreSend
	adapterStore  *mockAdapterStoreSend
	identityStore *mockAdapterIdentityStoreSend
}

// newSendFixture creates a fully wired test fixture with happy-path defaults.
func newSendFixture() *sendTestFixture {
	f := &sendTestFixture{
		tenantID:    uuid.Must(uuid.NewV7()),
		workspaceID: uuid.Must(uuid.NewV7()),
		sysWSID:     uuid.Must(uuid.NewV7()),
		templateID:  uuid.Must(uuid.NewV7()),
		versionID:   uuid.Must(uuid.NewV7()),
		typeID:      uuid.Must(uuid.NewV7()),
		adapterID:   uuid.Must(uuid.NewV7()),
	}

	f.tenantStore = &mockTenantStoreSend{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			if code == "latam" {
				return &domain.Tenant{ID: f.tenantID, Code: "latam", Name: "LATAM"}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	f.wsStore = &mockWorkspaceStoreSend{
		getByTenantAndCodeFn: func(_ context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
			if tenantID == f.tenantID && code == "acme" {
				return &domain.Workspace{ID: f.workspaceID, TenantID: f.tenantID, Code: "acme", Name: "Acme"}, nil
			}
			return nil, domain.ErrNotFound
		},
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			if id == f.workspaceID {
				return &domain.Workspace{ID: f.workspaceID, TenantID: f.tenantID, Code: "acme", Name: "Acme"}, nil
			}
			return nil, domain.ErrNotFound
		},
		getSystemWorkspaceFn: func(_ context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
			if tenantID == f.tenantID {
				return &domain.Workspace{ID: f.sysWSID, TenantID: f.tenantID, Code: "_system", Name: "System", IsSystem: true}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	f.templateStore = &mockTemplateStoreSend{
		getTypeBySlugFn: func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			if slug == "welcome" {
				return &domain.TemplateType{
					ID:        f.typeID,
					Slug:      "welcome",
					Name:      "Welcome Email",
					AdapterID: &f.adapterID,
				}, nil
			}
			return nil, domain.ErrTemplateTypeNotFound
		},
		resolveTemplateFn: func(_ context.Context, typeID uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
			if typeID == f.typeID {
				return &domain.Template{
					ID:             f.templateID,
					TemplateTypeID: f.typeID,
				}, nil
			}
			return nil, domain.ErrTemplateNotFound
		},
		getPublishedVersionFn: func(_ context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
			if templateID == f.templateID {
				return &domain.TemplateVersion{
					ID:            f.versionID,
					TemplateID:    f.templateID,
					VersionNumber: 1,
					Status:        domain.VersionStatusPublished,
					Subject:       "Welcome {{ event.name }}",
					PreviewText:   "Welcome to our platform",
					FromName:      "{{ injector.brand.name }}",
					BodyMJML:      "<mj-text>Hello {{ event.name }}</mj-text>",
					DefaultLocale: "en",
				}, nil
			}
			return nil, domain.ErrNoPublishedVersion
		},
	}

	f.injectorStore = &mockInjectorStoreSend{
		listDefinitionsInChainFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return nil, nil
		},
	}

	f.adapterStore = &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id == f.adapterID {
				return &domain.Adapter{
					ID:          f.adapterID,
					Name:        "SES Default",
					AdapterType: domain.AdapterTypeSES,
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	f.identityStore = &mockAdapterIdentityStoreSend{
		getDefaultFn: func(_ context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error) {
			if adapterID == f.adapterID {
				return &domain.AdapterIdentity{
					ID:             uuid.Must(uuid.NewV7()),
					AdapterID:      f.adapterID,
					Identity:       "hello@example.com",
					IdentityType:   domain.IdentityTypeEmail,
					Status:         domain.IdentityStatusVerified,
					SendingEnabled: true,
					IsDefault:      true,
					Source:         domain.IdentitySourceManual,
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	f.emailStore = &mockEmailStoreSend{}
	f.suppression = &mockSuppressionStoreSend{}
	f.jq = &mockJobQueueSend{}
	f.cache = newMockCacheSend()

	return f
}

func (f *sendTestFixture) buildService() *service.SendService {
	chainResolver := resolution.NewChainResolver(f.wsStore, f.cache)
	templateResolver := resolution.NewTemplateResolver(f.templateStore, f.cache, chainResolver)
	injectorMerger := resolution.NewInjectorMerger(f.injectorStore, chainResolver)
	adapterResolver := resolution.NewAdapterResolver(f.adapterStore, f.cache)
	renderer := service.NewVariableRenderer()
	identitySvc := service.NewIdentityService(f.identityStore, f.adapterStore, nil, nil)

	return service.NewSendService(
		templateResolver,
		injectorMerger,
		adapterResolver,
		identitySvc,
		f.emailStore,
		f.suppression,
		f.jq,
		renderer,
		f.tenantStore,
		f.wsStore,
		nil,
	)
}

func (f *sendTestFixture) happyRequest() *service.SendRequest {
	return &service.SendRequest{
		Ref:       "latam:acme:welcome",
		To:        []string{"alice@user.com"},
		Variables: map[string]any{"name": "Alice"},
	}
}

// --- Tests ---

func TestSendService_HappyPath(t *testing.T) {
	f := newSendFixture()
	var enqueuedJobs []*port.SendJob
	f.jq.enqueueSendFn = func(_ context.Context, job *port.SendJob) error {
		enqueuedJobs = append(enqueuedJobs, job)
		return nil
	}

	svc := f.buildService()
	resp, err := svc.Send(context.Background(), f.happyRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "accepted" {
		t.Fatalf("expected status 'accepted', got %q", resp.Status)
	}
	if len(resp.TrackingIDs) != 1 {
		t.Fatalf("expected 1 tracking entry, got %d", len(resp.TrackingIDs))
	}
	if resp.TrackingIDs[0].To != "alice@user.com" {
		t.Fatalf("expected to 'alice@user.com', got %q", resp.TrackingIDs[0].To)
	}
	if !strings.HasPrefix(resp.TrackingIDs[0].TrackingID, "trk_") {
		t.Fatalf("expected tracking ID prefix 'trk_', got %q", resp.TrackingIDs[0].TrackingID)
	}
	if resp.TrackingIDs[0].Status != "accepted" {
		t.Fatalf("expected tracking entry status 'accepted', got %q", resp.TrackingIDs[0].Status)
	}
	if resp.TemplateVersion != 1 {
		t.Fatalf("expected template version 1, got %d", resp.TemplateVersion)
	}
	if resp.TemplateResolved != "latam:acme:welcome" {
		t.Fatalf("expected template resolved 'latam:acme:welcome', got %q", resp.TemplateResolved)
	}

	// Verify email was created
	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email created, got %d", len(f.emailStore.emails))
	}
	email := f.emailStore.emails[0]
	if email.Status != domain.StatusQueued {
		t.Fatalf("expected status 'queued', got %q", email.Status)
	}
	if email.RecipientEmail != "alice@user.com" {
		t.Fatalf("expected recipient 'alice@user.com', got %q", email.RecipientEmail)
	}
	if email.FromEmail != "hello@example.com" {
		t.Fatalf("expected from_email 'hello@example.com', got %q", email.FromEmail)
	}
	if email.SubjectRendered != "Welcome Alice" {
		t.Fatalf("expected subject 'Welcome Alice', got %q", email.SubjectRendered)
	}
	if email.TemplateRef != "latam:acme:welcome" {
		t.Fatalf("expected template ref 'latam:acme:welcome', got %q", email.TemplateRef)
	}
	if email.WorkspaceID != f.workspaceID {
		t.Fatalf("expected workspace ID %s, got %s", f.workspaceID, email.WorkspaceID)
	}
	if email.TenantID != f.tenantID {
		t.Fatalf("expected tenant ID %s, got %s", f.tenantID, email.TenantID)
	}

	// Verify job was enqueued
	if len(enqueuedJobs) != 1 {
		t.Fatalf("expected 1 job enqueued, got %d", len(enqueuedJobs))
	}
	if enqueuedJobs[0].EmailID != email.ID {
		t.Fatalf("expected job email ID %s, got %s", email.ID, enqueuedJobs[0].EmailID)
	}
	if enqueuedJobs[0].AdapterID != f.adapterID {
		t.Fatalf("expected adapter ID %s, got %s", f.adapterID, enqueuedJobs[0].AdapterID)
	}
}

func TestSendService_MultipleRecipients(t *testing.T) {
	f := newSendFixture()
	var enqueuedJobs []*port.SendJob
	f.jq.enqueueSendFn = func(_ context.Context, job *port.SendJob) error {
		enqueuedJobs = append(enqueuedJobs, job)
		return nil
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"alice@user.com", "bob@user.com", "charlie@user.com"}

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.TrackingIDs) != 3 {
		t.Fatalf("expected 3 tracking entries, got %d", len(resp.TrackingIDs))
	}
	if len(f.emailStore.emails) != 3 {
		t.Fatalf("expected 3 emails created, got %d", len(f.emailStore.emails))
	}
	if len(enqueuedJobs) != 3 {
		t.Fatalf("expected 3 jobs enqueued, got %d", len(enqueuedJobs))
	}

	// Verify unique tracking IDs
	seen := make(map[string]bool)
	for _, entry := range resp.TrackingIDs {
		if seen[entry.TrackingID] {
			t.Fatalf("duplicate tracking ID: %s", entry.TrackingID)
		}
		seen[entry.TrackingID] = true
	}
}

func TestSendService_SuppressedRecipient(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		if email == "bounced@user.com" {
			return true, "hard_bounce", nil
		}
		return false, "", nil
	}

	var enqueuedJobs []*port.SendJob
	f.jq.enqueueSendFn = func(_ context.Context, job *port.SendJob) error {
		enqueuedJobs = append(enqueuedJobs, job)
		return nil
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"bounced@user.com"}

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.TrackingIDs) != 1 {
		t.Fatalf("expected 1 tracking entry, got %d", len(resp.TrackingIDs))
	}
	if resp.TrackingIDs[0].Status != "suppressed" {
		t.Fatalf("expected tracking entry status 'suppressed', got %q", resp.TrackingIDs[0].Status)
	}

	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email created, got %d", len(f.emailStore.emails))
	}
	if f.emailStore.emails[0].Status != domain.StatusSuppressed {
		t.Fatalf("expected status 'suppressed', got %q", f.emailStore.emails[0].Status)
	}

	if len(enqueuedJobs) != 0 {
		t.Fatalf("expected 0 jobs enqueued for suppressed recipient, got %d", len(enqueuedJobs))
	}

	if len(f.emailStore.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(f.emailStore.events))
	}
	if f.emailStore.events[0].EventType != domain.EventTypeSuppressed {
		t.Fatalf("expected event type 'suppressed', got %q", f.emailStore.events[0].EventType)
	}
}

func TestSendService_MixedSuppressedAndActive(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		if email == "bounced@user.com" {
			return true, "hard_bounce", nil
		}
		return false, "", nil
	}

	var enqueuedJobs []*port.SendJob
	f.jq.enqueueSendFn = func(_ context.Context, job *port.SendJob) error {
		enqueuedJobs = append(enqueuedJobs, job)
		return nil
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"alice@user.com", "bounced@user.com"}

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.TrackingIDs) != 2 {
		t.Fatalf("expected 2 tracking entries, got %d", len(resp.TrackingIDs))
	}
	if len(f.emailStore.emails) != 2 {
		t.Fatalf("expected 2 emails created, got %d", len(f.emailStore.emails))
	}
	if len(enqueuedJobs) != 1 {
		t.Fatalf("expected 1 job enqueued, got %d", len(enqueuedJobs))
	}
}

func TestSendService_InvalidRef(t *testing.T) {
	f := newSendFixture()
	svc := f.buildService()

	req := f.happyRequest()
	req.Ref = "invalid-ref"

	_, err := svc.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid ref")
	}
	if !errors.Is(err, domain.ErrInvalidRef) {
		t.Fatalf("expected ErrInvalidRef, got %v", err)
	}
}

func TestSendService_TenantNotFound(t *testing.T) {
	f := newSendFixture()
	f.tenantStore.getByCodeFn = func(_ context.Context, _ string) (*domain.Tenant, error) {
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error for missing tenant")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSendService_WorkspaceNotFound(t *testing.T) {
	f := newSendFixture()
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, _ uuid.UUID, _ string) (*domain.Workspace, error) {
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSendService_TemplateNotFound(t *testing.T) {
	f := newSendFixture()
	f.templateStore.getTypeBySlugFn = func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
		return nil, domain.ErrTemplateTypeNotFound
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error for missing template type")
	}
	if !errors.Is(err, domain.ErrTemplateTypeNotFound) {
		t.Fatalf("expected ErrTemplateTypeNotFound, got %v", err)
	}
}

func TestSendService_NoAdapterConfigured(t *testing.T) {
	f := newSendFixture()
	f.templateStore.getTypeBySlugFn = func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
		return &domain.TemplateType{
			ID:        f.typeID,
			Slug:      slug,
			Name:      "Welcome",
			AdapterID: nil,
		}, nil
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error for no adapter configured")
	}
	if !errors.Is(err, domain.ErrNoAdapterConfigured) {
		t.Fatalf("expected ErrNoAdapterConfigured, got %v", err)
	}
}

func TestSendService_WithExternalID(t *testing.T) {
	f := newSendFixture()
	svc := f.buildService()

	req := f.happyRequest()
	extID := "ext-123"
	req.ExternalID = &extID

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ExternalID == nil || *resp.ExternalID != "ext-123" {
		t.Fatalf("expected external ID 'ext-123', got %v", resp.ExternalID)
	}

	if f.emailStore.emails[0].ExternalID == nil || *f.emailStore.emails[0].ExternalID != "ext-123" {
		t.Fatal("expected email to have external ID")
	}
}

func TestSendService_WithCCAndBCC(t *testing.T) {
	f := newSendFixture()
	svc := f.buildService()

	req := f.happyRequest()
	req.CC = []string{"cc@user.com"}
	req.BCC = []string{"bcc@user.com"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	email := f.emailStore.emails[0]
	if len(email.CC) != 1 || email.CC[0] != "cc@user.com" {
		t.Fatalf("expected CC ['cc@user.com'], got %v", email.CC)
	}
	if len(email.BCC) != 1 || email.BCC[0] != "bcc@user.com" {
		t.Fatalf("expected BCC ['bcc@user.com'], got %v", email.BCC)
	}
}

func TestSendService_WithLocale(t *testing.T) {
	f := newSendFixture()

	localeSubject := "Bienvenido {{ event.name }}"
	f.templateStore.getLocaleFn = func(_ context.Context, _ uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
		if locale == "es" {
			return &domain.TemplateVersionLocale{
				ID:      uuid.Must(uuid.NewV7()),
				Locale:  "es",
				Subject: &localeSubject,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	req := f.happyRequest()
	locale := "es"
	req.Locale = &locale

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	email := f.emailStore.emails[0]
	if email.SubjectRendered != "Bienvenido Alice" {
		t.Fatalf("expected subject 'Bienvenido Alice', got %q", email.SubjectRendered)
	}
	if email.Locale == nil || *email.Locale != "es" {
		t.Fatalf("expected locale 'es', got %v", email.Locale)
	}
}

func TestSendService_TemplateDisabled(t *testing.T) {
	f := newSendFixture()
	f.templateStore.resolveTemplateFn = func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
		return &domain.Template{
			ID:             f.templateID,
			TemplateTypeID: f.typeID,
			IsDisabled:     true,
		}, nil
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error for disabled template")
	}
	if !errors.Is(err, domain.ErrTemplateDisabled) {
		t.Fatalf("expected ErrTemplateDisabled, got %v", err)
	}
}

func TestSendService_NoPublishedVersion(t *testing.T) {
	f := newSendFixture()
	f.templateStore.getPublishedVersionFn = func(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
		return nil, domain.ErrNoPublishedVersion
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error for no published version")
	}
	if !errors.Is(err, domain.ErrNoPublishedVersion) {
		t.Fatalf("expected ErrNoPublishedVersion, got %v", err)
	}
}

func TestSendService_EmailStoreCreateError(t *testing.T) {
	f := newSendFixture()
	dbErr := errors.New("db write failed")
	f.emailStore.createFn = func(_ context.Context, _ *domain.Email) error {
		return dbErr
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error when email store fails")
	}
	// When all recipients fail, the error is wrapped.
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped 'db write failed', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "all recipients failed") {
		t.Fatalf("expected 'all recipients failed' wrapper, got %q", err.Error())
	}
}

func TestSendService_QueueEnqueueError(t *testing.T) {
	f := newSendFixture()
	queueErr := errors.New("queue unavailable")
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error {
		return queueErr
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error when queue fails")
	}
	// When all recipients fail, the error is wrapped.
	if !errors.Is(err, queueErr) {
		t.Fatalf("expected wrapped 'queue unavailable', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "all recipients failed") {
		t.Fatalf("expected 'all recipients failed' wrapper, got %q", err.Error())
	}
}

func TestSendService_SuppressionStoreError(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, _ string) (bool, string, error) {
		return false, "", errors.New("suppression store unavailable")
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error when suppression store fails")
	}
	if !strings.Contains(err.Error(), "check suppression") {
		t.Fatalf("expected 'check suppression' in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "suppression store unavailable") {
		t.Fatalf("expected wrapped cause 'suppression store unavailable', got %q", err.Error())
	}
}

func TestSendService_PartialFailure_SomeRecipientsSucceed(t *testing.T) {
	f := newSendFixture()
	callCount := 0
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error {
		callCount++
		if callCount == 2 {
			return errors.New("queue full")
		}
		return nil
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"alice@user.com", "bob@user.com", "charlie@user.com"}

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error for partial failure, got %v", err)
	}

	if resp.Status != "partial" {
		t.Fatalf("expected status 'partial', got %q", resp.Status)
	}
	if len(resp.TrackingIDs) != 3 {
		t.Fatalf("expected 3 tracking entries, got %d", len(resp.TrackingIDs))
	}

	// bob@user.com (index 1) should be "failed".
	if resp.TrackingIDs[1].Status != "failed" {
		t.Fatalf("expected bob's status 'failed', got %q", resp.TrackingIDs[1].Status)
	}
	if resp.TrackingIDs[1].Error == "" {
		t.Fatal("expected bob's error message to be set")
	}

	// alice and charlie should be "accepted".
	if resp.TrackingIDs[0].Status != "accepted" {
		t.Fatalf("expected alice's status 'accepted', got %q", resp.TrackingIDs[0].Status)
	}
	if resp.TrackingIDs[2].Status != "accepted" {
		t.Fatalf("expected charlie's status 'accepted', got %q", resp.TrackingIDs[2].Status)
	}
}

func TestSendService_NoDefaultIdentity(t *testing.T) {
	f := newSendFixture()
	f.identityStore.getDefaultFn = func(_ context.Context, _ uuid.UUID) (*domain.AdapterIdentity, error) {
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err == nil {
		t.Fatal("expected error when no default identity")
	}
	if !errors.Is(err, domain.ErrNoDefaultIdentity) {
		t.Fatalf("expected ErrNoDefaultIdentity, got %v", err)
	}
}

func TestSendService_ScopeMismatch(t *testing.T) {
	f := newSendFixture()
	svc := f.buildService()

	req := f.happyRequest()
	req.AuthWorkspaceID = uuid.Must(uuid.NewV7())

	_, err := svc.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected scope mismatch error")
	}
	if !errors.Is(err, domain.ErrWorkspaceScopeMismatch) {
		t.Fatalf("expected ErrWorkspaceScopeMismatch, got %v", err)
	}
}

func TestSendService_ScopeMatch(t *testing.T) {
	f := newSendFixture()
	svc := f.buildService()

	req := f.happyRequest()
	req.AuthWorkspaceID = f.workspaceID

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendService_SystemWorkspaceBlocked(t *testing.T) {
	f := newSendFixture()
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error) {
		if tenantID == f.tenantID && code == "_system" {
			return &domain.Workspace{
				ID:       f.sysWSID,
				TenantID: f.tenantID,
				Code:     "_system",
				Name:     "System",
				IsSystem: true,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.Ref = "latam:_system:welcome"
	req.AuthWorkspaceID = f.sysWSID

	_, err := svc.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected system workspace block error")
	}
	if !errors.Is(err, domain.ErrSystemWorkspaceBlocked) {
		t.Fatalf("expected ErrSystemWorkspaceBlocked, got %v", err)
	}
}

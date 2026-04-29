package service_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
	"github.com/rendis/senda/internal/service"
)

// --- Send-specific mocks (suffixed to avoid collisions with other test files) ---

type mockTenantStoreSend struct {
	getByIDFn   func(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	getByCodeFn func(ctx context.Context, code string) (*domain.Tenant, error)
}

func (m *mockTenantStoreSend) Create(_ context.Context, _ *domain.Tenant) error { return nil }
func (m *mockTenantStoreSend) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
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
	getByTenantAndCodeFn func(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error)
	getByIDFn            func(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	getSystemWorkspaceFn func(ctx context.Context, tenantID uuid.UUID, environment domain.Environment) (*domain.Workspace, error)
	listByTenantFn       func(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error)
}

func (m *mockWorkspaceStoreSend) Create(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStoreSend) CreateLogicalPair(_ context.Context, _ *domain.Workspace, _ *domain.Workspace) error {
	return nil
}
func (m *mockWorkspaceStoreSend) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStoreSend) GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
	if m.getByTenantAndCodeFn != nil {
		return m.getByTenantAndCodeFn(ctx, tenantID, code, environment)
	}
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStoreSend) GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID, environment domain.Environment) (*domain.Workspace, error) {
	if m.getSystemWorkspaceFn != nil {
		return m.getSystemWorkspaceFn(ctx, tenantID, environment)
	}
	return nil, domain.ErrNotFound
}
func (m *mockWorkspaceStoreSend) ListByTenant(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, opts port.ListOptions) ([]*domain.Workspace, string, error) {
	if m.listByTenantFn != nil {
		return m.listByTenantFn(ctx, tenantID, environment, opts)
	}
	return nil, "", nil
}
func (m *mockWorkspaceStoreSend) UpdateShared(_ context.Context, _ uuid.UUID, _, _, _ string) error {
	return nil
}
func (m *mockWorkspaceStoreSend) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStoreSend) SoftDeleteLogical(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockWorkspaceStoreSend) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockWorkspaceStoreSend) GetUnsubscribeSigningKey(_ context.Context, _ uuid.UUID) ([]byte, error) {
	return make([]byte, 32), nil
}

type mockEmailStoreSend struct {
	createFn     func(ctx context.Context, email *domain.Email) error
	addEventFn   func(ctx context.Context, event *domain.EmailEvent) error
	getPayloadFn func(ctx context.Context, emailID uuid.UUID) (*domain.EmailPayload, error)
	emails       []*domain.Email
	events       []*domain.EmailEvent
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
func (m *mockEmailStoreSend) GetPayload(ctx context.Context, emailID uuid.UUID) (*domain.EmailPayload, error) {
	if m.getPayloadFn != nil {
		return m.getPayloadFn(ctx, emailID)
	}
	return nil, nil
}
func (m *mockEmailStoreSend) PurgeWorkspaceRuntime(_ context.Context, _ uuid.UUID) error { return nil }
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
func (m *mockEmailStoreSend) AddEventTx(_ context.Context, _ pgx.Tx, event *domain.EmailEvent) error {
	m.events = append(m.events, event)
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
	isSuppressedFn       func(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error)
	checkBatchFn         func(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]string, error)
	getStatusesFn        func(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]port.SuppressionStatus, error)
	getStatusesCalls     int
	lastGetStatusesInput []string
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
func (m *mockSuppressionStoreSend) GetSuppressionStatuses(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]port.SuppressionStatus, error) {
	m.getStatusesCalls++
	m.lastGetStatusesInput = append([]string(nil), emails...)
	if m.getStatusesFn != nil {
		return m.getStatusesFn(ctx, wsID, emails)
	}
	if m.checkBatchFn != nil {
		batch, err := m.checkBatchFn(ctx, wsID, emails)
		if err != nil {
			return nil, err
		}
		statuses := make(map[string]port.SuppressionStatus, len(batch))
		for email, reason := range batch {
			statuses[email] = port.SuppressionStatus{Suppressed: true, Reason: reason}
		}
		return statuses, nil
	}

	statuses := make(map[string]port.SuppressionStatus, len(emails))
	for _, email := range emails {
		suppressed, reason, err := m.IsSuppressed(ctx, wsID, email)
		if err != nil {
			return nil, err
		}
		statuses[email] = port.SuppressionStatus{
			Suppressed: suppressed,
			Reason:     reason,
		}
	}
	return statuses, nil
}

func (m *mockSuppressionStoreSend) CheckBatch(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]string, error) {
	if m.checkBatchFn != nil {
		return m.checkBatchFn(ctx, wsID, emails)
	}
	statuses, err := m.GetSuppressionStatuses(ctx, wsID, emails)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(statuses))
	for email, status := range statuses {
		if status.Suppressed {
			result[email] = status.Reason
		}
	}
	return result, nil
}
func (m *mockSuppressionStoreSend) GetActiveWorkspaceSuppression(_ context.Context, _ uuid.UUID, _ string) (*domain.SuppressionWorkspace, error) {
	return nil, nil
}
func (m *mockSuppressionStoreSend) RemoveWorkspaceSuppression(_ context.Context, _ uuid.UUID, _ string, _ string) error {
	return nil
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
	cloneVersionFn          func(ctx context.Context, templateID, sourceVersionID uuid.UUID, createdBy *uuid.UUID) (*domain.TemplateVersion, error)
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
func (m *mockTemplateStoreSend) ForkTemplate(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ *uuid.UUID) (*domain.Template, error) {
	return nil, domain.ErrNotFound
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
func (m *mockTemplateStoreSend) CloneVersion(ctx context.Context, templateID, sourceVersionID uuid.UUID, createdBy *uuid.UUID) (*domain.TemplateVersion, error) {
	if m.cloneVersionFn != nil {
		return m.cloneVersionFn(ctx, templateID, sourceVersionID, createdBy)
	}
	return nil, nil
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
func (m *mockTemplateStoreSend) GetLatestVersion(_ context.Context, _ uuid.UUID) (*domain.TemplateVersion, error) {
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStoreSend) SoftDeleteTemplate(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockTemplateStoreSend) DeleteDraftVersion(_ context.Context, _ uuid.UUID) error { return nil }
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
func (m *mockTemplateStoreSend) UpdateType(_ context.Context, _ *domain.TemplateType) error {
	return nil
}
func (m *mockTemplateStoreSend) SoftDeleteType(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockTemplateStoreSend) GetTemplateByID(_ context.Context, _ uuid.UUID) (*domain.Template, error) {
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStoreSend) GetTypeByID(_ context.Context, _ uuid.UUID) (*domain.TemplateType, error) {
	return nil, domain.ErrNotFound
}

type mockInjectorStoreSend struct {
	listDefinitionsInChainFn func(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)
	getFieldsByDefinitionFn  func(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)
	getValuesFn              func(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)
}

func (m *mockInjectorStoreSend) CreateDefinition(_ context.Context, _ *domain.InjectorDefinition) error {
	return nil
}
func (m *mockInjectorStoreSend) UpdateDefinitionSchema(_ context.Context, _ string, _ *uuid.UUID, _ *domain.InjectorDefinition, _ []*domain.InjectorField) error {
	return nil
}
func (m *mockInjectorStoreSend) GetDefinitionByID(_ context.Context, _ uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStoreSend) FindDefinitionByName(_ context.Context, _ string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStoreSend) SoftDeleteDefinition(_ context.Context, _ uuid.UUID) error {
	return nil
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
func (m *mockInjectorStoreSend) UpdateField(_ context.Context, _ *domain.InjectorField) error {
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
func (m *mockInjectorStoreSend) GetAllFieldsByDefinitions(ctx context.Context, defIDs []uuid.UUID) (map[uuid.UUID][]*domain.InjectorField, error) {
	result := make(map[uuid.UUID][]*domain.InjectorField, len(defIDs))
	for _, defID := range defIDs {
		fields, err := m.GetFieldsByDefinition(ctx, defID)
		if err != nil {
			return nil, err
		}
		result[defID] = fields
	}
	return result, nil
}
func (m *mockInjectorStoreSend) GetAllValuesByDefinitions(_ context.Context, _ []uuid.UUID, _ []uuid.NullUUID) (map[uuid.UUID][]*domain.InjectorValue, error) {
	return nil, nil
}

type mockAdapterIdentityStoreSend struct {
	createFn        func(ctx context.Context, identity *domain.AdapterIdentity) error
	getByIDFn       func(ctx context.Context, id uuid.UUID) (*domain.AdapterIdentity, error)
	getDefaultFn    func(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error)
	listByAdapterFn func(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error)
}

func (m *mockAdapterIdentityStoreSend) Create(ctx context.Context, identity *domain.AdapterIdentity) error {
	if m.createFn != nil {
		return m.createFn(ctx, identity)
	}
	return nil
}
func (m *mockAdapterIdentityStoreSend) GetByID(ctx context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrIdentityNotFound
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
		getByTenantAndCodeFn: func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
			if tenantID == f.tenantID && code == "acme" {
				return &domain.Workspace{
					ID:          f.workspaceID,
					TenantID:    f.tenantID,
					Code:        "acme",
					Name:        "Acme",
					Environment: domain.EnvironmentProd,
				}, nil
			}
			return nil, domain.ErrNotFound
		},
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			if id == f.workspaceID {
				return &domain.Workspace{
					ID:          f.workspaceID,
					TenantID:    f.tenantID,
					Code:        "acme",
					Name:        "Acme",
					Environment: domain.EnvironmentProd,
				}, nil
			}
			if id == f.sysWSID {
				return &domain.Workspace{
					ID:          f.sysWSID,
					TenantID:    f.tenantID,
					Code:        "_system",
					Name:        "System",
					IsSystem:    true,
					Environment: domain.EnvironmentProd,
				}, nil
			}
			return nil, domain.ErrNotFound
		},
		getSystemWorkspaceFn: func(_ context.Context, tenantID uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
			if tenantID == f.tenantID {
				return &domain.Workspace{
					ID:          f.sysWSID,
					TenantID:    f.tenantID,
					Code:        "_system",
					Name:        "System",
					IsSystem:    true,
					Environment: domain.EnvironmentProd,
				}, nil
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
	return f.buildServiceWithCodeInjectors(nil, nil)
}

func (f *sendTestFixture) buildServiceWithCodeInjectors(codeInjectors []port.CodeInjector, initFunc port.CodeInitFunc) *service.SendService {
	chainResolver := resolution.NewChainResolver(f.wsStore, f.cache)
	templateResolver := resolution.NewTemplateResolver(f.templateStore, f.cache, chainResolver)
	injectorMerger := resolution.NewInjectorMerger(f.injectorStore, chainResolver, nil, codeInjectors, initFunc)
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
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, _ uuid.UUID, _ string, _ domain.Environment) (*domain.Workspace, error) {
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
	if !strings.Contains(err.Error(), "evaluate suppression batch") {
		t.Fatalf("expected 'evaluate suppression batch' in error, got %q", err.Error())
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

// --- C9: CC/BCC suppression tests ---

func TestSendService_SuppressedCC_FilteredOut(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		if email == "suppressed-cc@user.com" {
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
	req.CC = []string{"clean-cc@user.com", "suppressed-cc@user.com"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email created, got %d", len(f.emailStore.emails))
	}
	email := f.emailStore.emails[0]

	// suppressed-cc@user.com must be filtered out, only clean-cc@user.com remains
	if len(email.CC) != 1 {
		t.Fatalf("expected 1 CC after suppression filter, got %d: %v", len(email.CC), email.CC)
	}
	if email.CC[0] != "clean-cc@user.com" {
		t.Fatalf("expected CC[0]='clean-cc@user.com', got %q", email.CC[0])
	}

	// job still enqueued for the To recipient
	if len(enqueuedJobs) != 1 {
		t.Fatalf("expected 1 job enqueued, got %d", len(enqueuedJobs))
	}
}

func TestSendService_SuppressedBCC_FilteredOut(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		if email == "suppressed-bcc@user.com" {
			return true, "complained", nil
		}
		return false, "", nil
	}

	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	req := f.happyRequest()
	req.BCC = []string{"suppressed-bcc@user.com", "clean-bcc@user.com"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email created, got %d", len(f.emailStore.emails))
	}
	email := f.emailStore.emails[0]

	// suppressed-bcc@user.com must be filtered out, only clean-bcc@user.com remains
	if len(email.BCC) != 1 {
		t.Fatalf("expected 1 BCC after suppression filter, got %d: %v", len(email.BCC), email.BCC)
	}
	if email.BCC[0] != "clean-bcc@user.com" {
		t.Fatalf("expected BCC[0]='clean-bcc@user.com', got %q", email.BCC[0])
	}
}

// --- C8: Suppressed path atomicity tests ---

// TestSendService_SuppressedPath_BothRecordAndEventExist verifies that sending
// to a suppressed recipient creates both the email record (StatusSuppressed) and
// the suppression event, so the two writes are never split across partial failures.
func TestSendService_SuppressedPath_BothRecordAndEventExist(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		if email == "suppressed@user.com" {
			return true, "hard_bounce", nil
		}
		return false, "", nil
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"suppressed@user.com"}

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Email record must exist with suppressed status.
	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email record, got %d", len(f.emailStore.emails))
	}
	if f.emailStore.emails[0].Status != domain.StatusSuppressed {
		t.Fatalf("expected email status %q, got %q", domain.StatusSuppressed, f.emailStore.emails[0].Status)
	}

	// Suppression event must also exist.
	if len(f.emailStore.events) != 1 {
		t.Fatalf("expected 1 suppression event, got %d", len(f.emailStore.events))
	}
	evt := f.emailStore.events[0]
	if evt.EventType != domain.EventTypeSuppressed {
		t.Fatalf("expected event type %q, got %q", domain.EventTypeSuppressed, evt.EventType)
	}
	if evt.EmailID != f.emailStore.emails[0].ID {
		t.Fatalf("event email_id %s does not match email id %s", evt.EmailID, f.emailStore.emails[0].ID)
	}
	// Suppression reason must be captured in metadata.
	reason, ok := evt.Metadata["reason"]
	if !ok {
		t.Fatal("expected 'reason' key in suppression event metadata")
	}
	if reason != "hard_bounce" {
		t.Fatalf("expected reason 'hard_bounce', got %v", reason)
	}

	// Response entry must have status "suppressed".
	if len(resp.TrackingIDs) != 1 || resp.TrackingIDs[0].Status != "suppressed" {
		t.Fatalf("expected tracking status 'suppressed', got %v", resp.TrackingIDs)
	}
}

// TestSendService_SuppressedPath_AddEventFailure_EmailStillAccepted verifies that
// when AddEvent fails during the suppressed path (pool=nil fallback), the error is
// non-fatal: the email record is still created and the response reports "suppressed".
// This documents the current best-effort behaviour of the non-transactional fallback.
func TestSendService_SuppressedPath_AddEventFailure_EmailStillAccepted(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, _ string) (bool, string, error) {
		return true, "complaint", nil
	}
	f.emailStore.addEventFn = func(_ context.Context, _ *domain.EmailEvent) error {
		return errors.New("event store unavailable")
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"complained@user.com"}

	// AddEvent failure must NOT propagate — it is logged and swallowed.
	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: AddEvent failure on suppressed path must be non-fatal, got: %v", err)
	}

	// Email record must still exist.
	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email record despite AddEvent failure, got %d", len(f.emailStore.emails))
	}

	// Response entry must still be "suppressed".
	if len(resp.TrackingIDs) != 1 || resp.TrackingIDs[0].Status != "suppressed" {
		t.Fatalf("expected tracking status 'suppressed', got %v", resp.TrackingIDs)
	}
}

func TestSendService_AllCCBCC_Suppressed_EmailStillSent(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		if email == "suppressed-cc@user.com" || email == "suppressed-bcc@user.com" {
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
	req.CC = []string{"suppressed-cc@user.com"}
	req.BCC = []string{"suppressed-bcc@user.com"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email created (To recipient), got %d", len(f.emailStore.emails))
	}
	email := f.emailStore.emails[0]

	// CC and BCC should be empty after all entries suppressed
	if len(email.CC) != 0 {
		t.Fatalf("expected empty CC, got %v", email.CC)
	}
	if len(email.BCC) != 0 {
		t.Fatalf("expected empty BCC, got %v", email.BCC)
	}

	// To recipient still gets the email
	if len(enqueuedJobs) != 1 {
		t.Fatalf("expected 1 job enqueued for To recipient, got %d", len(enqueuedJobs))
	}
}

// --- C10: Queued event test ---

func TestSendService_QueuedEventRecorded(t *testing.T) {
	f := newSendFixture()
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	resp, err := svc.Send(context.Background(), f.happyRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.TrackingIDs) != 1 || resp.TrackingIDs[0].Status != "accepted" {
		t.Fatalf("expected 1 accepted tracking entry, got %v", resp.TrackingIDs)
	}

	// At least one event must be EventTypeQueued for the created email
	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(f.emailStore.emails))
	}
	emailID := f.emailStore.emails[0].ID

	var queuedEvent *domain.EmailEvent
	for _, ev := range f.emailStore.events {
		if ev.EmailID == emailID && ev.EventType == domain.EventTypeQueued {
			queuedEvent = ev
			break
		}
	}
	if queuedEvent == nil {
		t.Fatalf("expected EventTypeQueued event for email %s, got events: %v", emailID, f.emailStore.events)
	}
}

func TestSendService_QueuedEvent_MultipleRecipients(t *testing.T) {
	f := newSendFixture()
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"alice@user.com", "bob@user.com"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.emailStore.emails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(f.emailStore.emails))
	}

	// Each email must have its own EventTypeQueued event
	for _, email := range f.emailStore.emails {
		found := false
		for _, ev := range f.emailStore.events {
			if ev.EmailID == email.ID && ev.EventType == domain.EventTypeQueued {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no EventTypeQueued event found for email %s (%s)", email.ID, email.RecipientEmail)
		}
	}
}

func TestSendService_ScopeMismatch(t *testing.T) {
	f := newSendFixture()
	otherWorkspaceID := uuid.Must(uuid.NewV7())
	f.wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		switch id {
		case f.workspaceID:
			return &domain.Workspace{
				ID:          f.workspaceID,
				TenantID:    f.tenantID,
				Code:        "acme",
				Name:        "Acme",
				Environment: domain.EnvironmentProd,
			}, nil
		case otherWorkspaceID:
			return &domain.Workspace{
				ID:          otherWorkspaceID,
				TenantID:    f.tenantID,
				Code:        "other",
				Name:        "Other",
				Environment: domain.EnvironmentProd,
			}, nil
		default:
			return nil, domain.ErrNotFound
		}
	}
	svc := f.buildService()

	req := f.happyRequest()
	req.AuthWorkspaceID = otherWorkspaceID

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
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
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

func TestSendService_TestEnvironmentWorkspaceRecipientPolicyReplace(t *testing.T) {
	f := newSendFixture()
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
		if tenantID == f.tenantID && code == "acme" && environment == domain.EnvironmentTest {
			return &domain.Workspace{
				ID:                     f.workspaceID,
				TenantID:               f.tenantID,
				Code:                   "acme",
				Name:                   "Acme",
				Environment:            domain.EnvironmentTest,
				TestRecipientMode:      domain.TestRecipientModeReplace,
				TestRecipientAddresses: []string{"qa@example.com"},
			}, nil
		}
		return nil, domain.ErrNotFound
	}
	f.wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		if id == f.workspaceID {
			return &domain.Workspace{
				ID:          f.workspaceID,
				TenantID:    f.tenantID,
				Code:        "acme",
				Name:        "Acme",
				Environment: domain.EnvironmentTest,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.AuthWorkspaceID = f.workspaceID
	req.To = []string{"real.user@example.com"}
	req.CC = []string{"cc@example.com"}
	req.BCC = []string{"bcc@example.com"}

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.TrackingIDs) != 1 {
		t.Fatalf("expected 1 tracking entry, got %d", len(resp.TrackingIDs))
	}
	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 persisted email, got %d", len(f.emailStore.emails))
	}
	if got := f.emailStore.emails[0].RecipientEmail; got != "qa@example.com" {
		t.Fatalf("expected replaced recipient qa@example.com, got %s", got)
	}
	if len(f.emailStore.emails[0].CC) != 0 || len(f.emailStore.emails[0].BCC) != 0 {
		t.Fatalf("expected replace mode to clear CC/BCC, got CC=%v BCC=%v", f.emailStore.emails[0].CC, f.emailStore.emails[0].BCC)
	}
}

func TestSendService_TestEnvironmentWorkspaceRecipientPolicyAppend(t *testing.T) {
	f := newSendFixture()
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
		if tenantID == f.tenantID && code == "acme" && environment == domain.EnvironmentTest {
			return &domain.Workspace{
				ID:                     f.workspaceID,
				TenantID:               f.tenantID,
				Code:                   "acme",
				Name:                   "Acme",
				Environment:            domain.EnvironmentTest,
				TestRecipientMode:      domain.TestRecipientModeAppend,
				TestRecipientAddresses: []string{"qa@example.com"},
			}, nil
		}
		return nil, domain.ErrNotFound
	}
	f.wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		if id == f.workspaceID {
			return &domain.Workspace{
				ID:          f.workspaceID,
				TenantID:    f.tenantID,
				Code:        "acme",
				Name:        "Acme",
				Environment: domain.EnvironmentTest,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.AuthWorkspaceID = f.workspaceID
	req.To = []string{"real.user@example.com"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.emailStore.emails) != 2 {
		t.Fatalf("expected 2 persisted emails, got %d", len(f.emailStore.emails))
	}
	if got := f.emailStore.emails[0].RecipientEmail; got != "real.user@example.com" {
		t.Fatalf("expected original recipient first, got %s", got)
	}
	if got := f.emailStore.emails[1].RecipientEmail; got != "qa@example.com" {
		t.Fatalf("expected appended recipient qa@example.com, got %s", got)
	}
}

func TestSendService_TestEnvironmentTemplateTypeRecipientPolicyOverride(t *testing.T) {
	f := newSendFixture()
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
		if tenantID == f.tenantID && code == "acme" && environment == domain.EnvironmentTest {
			return &domain.Workspace{
				ID:                     f.workspaceID,
				TenantID:               f.tenantID,
				Code:                   "acme",
				Name:                   "Acme",
				Environment:            domain.EnvironmentTest,
				TestRecipientMode:      domain.TestRecipientModeAppend,
				TestRecipientAddresses: []string{"workspace@example.com"},
			}, nil
		}
		return nil, domain.ErrNotFound
	}
	f.wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		if id == f.workspaceID {
			return &domain.Workspace{
				ID:          f.workspaceID,
				TenantID:    f.tenantID,
				Code:        "acme",
				Name:        "Acme",
				Environment: domain.EnvironmentTest,
			}, nil
		}
		return nil, domain.ErrNotFound
	}
	f.templateStore.getTypeBySlugFn = func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
		if slug == "welcome" {
			overrideMode := domain.TestRecipientModeReplace
			return &domain.TemplateType{
				ID:                     f.typeID,
				Slug:                   "welcome",
				Name:                   "Welcome Email",
				AdapterID:              &f.adapterID,
				TestRecipientMode:      &overrideMode,
				TestRecipientAddresses: []string{"template@example.com"},
			}, nil
		}
		return nil, domain.ErrTemplateTypeNotFound
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.AuthWorkspaceID = f.workspaceID
	req.To = []string{"real.user@example.com"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 persisted email, got %d", len(f.emailStore.emails))
	}
	if got := f.emailStore.emails[0].RecipientEmail; got != "template@example.com" {
		t.Fatalf("expected template override recipient, got %s", got)
	}
}

func TestSendService_TestEnvironmentWithoutConfiguredPolicyFailsClosed(t *testing.T) {
	f := newSendFixture()
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error) {
		if tenantID == f.tenantID && code == "acme" && environment == domain.EnvironmentTest {
			return &domain.Workspace{
				ID:                f.workspaceID,
				TenantID:          f.tenantID,
				Code:              "acme",
				Name:              "Acme",
				Environment:       domain.EnvironmentTest,
				TestRecipientMode: domain.TestRecipientModeReplace,
			}, nil
		}
		return nil, domain.ErrNotFound
	}
	f.wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		if id == f.workspaceID {
			return &domain.Workspace{
				ID:          f.workspaceID,
				TenantID:    f.tenantID,
				Code:        "acme",
				Name:        "Acme",
				Environment: domain.EnvironmentTest,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.AuthWorkspaceID = f.workspaceID
	req.To = []string{"real.user@example.com"}
	req.CC = []string{"cc@example.com"}
	req.BCC = []string{"bcc@example.com"}

	_, err := svc.Send(context.Background(), req)
	if !errors.Is(err, domain.ErrTestRecipientPolicyUnconfigured) {
		t.Fatalf("expected ErrTestRecipientPolicyUnconfigured, got %v", err)
	}
	if len(f.emailStore.emails) != 0 {
		t.Fatalf("expected no persisted emails when policy is missing, got %d", len(f.emailStore.emails))
	}
}

func TestSendService_SharedSESAccessRevokedAtRuntime(t *testing.T) {
	f := newSendFixture()
	senderIdentityID := uuid.Must(uuid.NewV7())

	f.templateStore.getTypeBySlugFn = func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
		if slug == "welcome" {
			return &domain.TemplateType{
				ID:               f.typeID,
				WorkspaceID:      &f.workspaceID,
				Slug:             "welcome",
				Name:             "Welcome Email",
				AdapterID:        &f.adapterID,
				SenderIdentityID: &senderIdentityID,
			}, nil
		}
		return nil, domain.ErrTemplateTypeNotFound
	}

	f.wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		switch id {
		case f.workspaceID:
			return &domain.Workspace{ID: f.workspaceID, TenantID: f.tenantID, Code: "acme", Name: "Acme"}, nil
		case f.sysWSID:
			return &domain.Workspace{ID: f.sysWSID, TenantID: f.tenantID, Code: "_system", Name: "System", IsSystem: true}, nil
		default:
			return nil, domain.ErrNotFound
		}
	}

	f.adapterStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
		if id == f.adapterID {
			return &domain.Adapter{
				ID:          f.adapterID,
				WorkspaceID: &f.sysWSID,
				Name:        "SES Shared",
				AdapterType: domain.AdapterTypeSES,
			}, nil
		}
		return nil, domain.ErrNotFound
	}

	f.identityStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
		if id == senderIdentityID {
			return &domain.AdapterIdentity{
				ID:           senderIdentityID,
				AdapterID:    f.adapterID,
				Identity:     "a@example.dev",
				IdentityType: domain.IdentityTypeEmail,
				Status:       domain.IdentityStatusVerified,
			}, nil
		}
		return nil, domain.ErrIdentityNotFound
	}

	accessSvc := service.NewAdapterAccessService(
		f.adapterStore,
		f.identityStore,
		f.wsStore,
		&mockAdapterGrantStore{},
		&mockIdentityGrantStore{
			hasIdentityWorkspaceGrantFn: func(_ context.Context, identityID, workspaceID uuid.UUID) (bool, error) {
				return false, nil
			},
		},
		&mockTemplateTypeUsageStore{},
	)

	svc := f.buildService()
	svc.SetAdapterAccessService(accessSvc)

	_, err := svc.Send(context.Background(), f.happyRequest())
	if !errors.Is(err, domain.ErrSenderIdentityAccessDenied) {
		t.Fatalf("expected ErrSenderIdentityAccessDenied, got %v", err)
	}
}

// --- Code Injector integration tests ---

type testCodeInjector struct {
	code   string
	fields map[string]any
}

func (t *testCodeInjector) Code() string { return t.code }
func (t *testCodeInjector) Resolve() (port.CodeResolveFunc, []string) {
	return func(_ context.Context, _ *port.InjectorContext) (map[string]any, error) {
		return t.fields, nil
	}, nil
}
func (t *testCodeInjector) IsCritical() bool       { return false }
func (t *testCodeInjector) Timeout() time.Duration { return 0 }

func TestSendService_WithCodeInjectors(t *testing.T) {
	f := newSendFixture()

	var capturedEmail *domain.Email
	f.emailStore.createFn = func(_ context.Context, email *domain.Email) error {
		capturedEmail = email
		return nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	codeInj := &testCodeInjector{
		code:   "student",
		fields: map[string]any{"name": "Alice", "grade": "A+"},
	}

	svc := f.buildServiceWithCodeInjectors([]port.CodeInjector{codeInj}, nil)
	req := f.happyRequest()

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %q", resp.Status)
	}

	// Verify code injector values appear in the injectors snapshot.
	if capturedEmail == nil {
		t.Fatal("email was not created")
	}
	snapshot := capturedEmail.InjectorsSnapshot
	studentFields, ok := snapshot["student"]
	if !ok {
		t.Fatal("student injector missing from email snapshot")
	}
	if studentFields["name"] != "Alice" {
		t.Errorf("student.name = %v, want Alice", studentFields["name"])
	}
	if studentFields["grade"] != "A+" {
		t.Errorf("student.grade = %v, want A+", studentFields["grade"])
	}
}

func TestSendService_WithHeaders(t *testing.T) {
	f := newSendFixture()
	f.emailStore.createFn = func(_ context.Context, _ *domain.Email) error { return nil }
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	var capturedHeader string
	headerInj := &testCodeInjector{code: "header_reader"}
	// Override to read headers from context.
	headerInj.fields = nil // will use resolveFn pattern instead

	// Use a proper injector that reads headers.
	type headerReader struct{}
	hr := &headerReader{}
	_ = hr

	codeInj := &stubCodeInjectorSend{
		code: "from_header",
		resolveFn: func(_ context.Context, injCtx *port.InjectorContext) (map[string]any, error) {
			capturedHeader = injCtx.Header("X-Case-Id")
			return map[string]any{"case_id": capturedHeader}, nil
		},
	}

	svc := f.buildServiceWithCodeInjectors([]port.CodeInjector{codeInj}, nil)
	req := f.happyRequest()
	req.Headers = map[string]string{"X-Case-Id": "case-42"}

	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedHeader != "case-42" {
		t.Errorf("header X-Case-Id = %q, want case-42", capturedHeader)
	}
}

func TestSendService_SendBatch_IsolatesPerItemInjectorContext(t *testing.T) {
	f := newSendFixture()

	var capturedEmails []*domain.Email
	f.emailStore.createFn = func(_ context.Context, email *domain.Email) error {
		capturedEmails = append(capturedEmails, email)
		return nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	codeInj := &stubCodeInjectorSend{
		code: "student",
		resolveFn: func(_ context.Context, injCtx *port.InjectorContext) (map[string]any, error) {
			return map[string]any{
				"name": injCtx.Variables()["name"],
			}, nil
		},
	}

	svc := f.buildServiceWithCodeInjectors([]port.CodeInjector{codeInj}, nil)
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "alice@user.com", Variables: map[string]any{"name": "Alice"}},
			{To: "bob@user.com", Variables: map[string]any{"name": "Bob"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %q", resp.Status)
	}
	if len(capturedEmails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(capturedEmails))
	}
	if capturedEmails[0].InjectorsSnapshot["student"]["name"] != "Alice" {
		t.Fatalf("expected first snapshot to use Alice, got %+v", capturedEmails[0].InjectorsSnapshot)
	}
	if capturedEmails[1].InjectorsSnapshot["student"]["name"] != "Bob" {
		t.Fatalf("expected second snapshot to use Bob, got %+v", capturedEmails[1].InjectorsSnapshot)
	}
}

func TestSendService_SendBatch_AmortizesSharedResolution(t *testing.T) {
	f := newSendFixture()

	var tenantLookups int
	var workspaceLookups int
	var templateTypeLookups int
	var templateLookups int
	var publishedVersionLookups int
	var adapterLookups int
	var defaultIdentityLookups int

	f.tenantStore.getByCodeFn = func(_ context.Context, code string) (*domain.Tenant, error) {
		tenantLookups++
		if code != "latam" {
			return nil, domain.ErrNotFound
		}
		return &domain.Tenant{ID: f.tenantID, Code: "latam", Name: "LATAM"}, nil
	}
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
		workspaceLookups++
		if tenantID != f.tenantID || code != "acme" {
			return nil, domain.ErrNotFound
		}
		return &domain.Workspace{
			ID:          f.workspaceID,
			TenantID:    f.tenantID,
			Code:        "acme",
			Name:        "Acme",
			Environment: domain.EnvironmentProd,
		}, nil
	}
	f.templateStore.getTypeBySlugFn = func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
		templateTypeLookups++
		if slug != "welcome" {
			return nil, domain.ErrTemplateTypeNotFound
		}
		return &domain.TemplateType{
			ID:        f.typeID,
			Slug:      "welcome",
			Name:      "Welcome Email",
			AdapterID: &f.adapterID,
		}, nil
	}
	f.templateStore.resolveTemplateFn = func(_ context.Context, typeID uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
		templateLookups++
		if typeID != f.typeID {
			return nil, domain.ErrTemplateNotFound
		}
		return &domain.Template{ID: f.templateID, TemplateTypeID: f.typeID}, nil
	}
	f.templateStore.getPublishedVersionFn = func(_ context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
		publishedVersionLookups++
		if templateID != f.templateID {
			return nil, domain.ErrNoPublishedVersion
		}
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
	f.adapterStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
		adapterLookups++
		if id != f.adapterID {
			return nil, domain.ErrNotFound
		}
		return &domain.Adapter{
			ID:          f.adapterID,
			Name:        "SES Default",
			AdapterType: domain.AdapterTypeSES,
		}, nil
	}
	f.identityStore.getDefaultFn = func(_ context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error) {
		defaultIdentityLookups++
		if adapterID != f.adapterID {
			return nil, domain.ErrNotFound
		}
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

	f.emailStore.createFn = func(_ context.Context, _ *domain.Email) error { return nil }
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "alice@user.com", Variables: map[string]any{"name": "Alice"}},
			{To: "bob@user.com", Variables: map[string]any{"name": "Bob"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %q", resp.Status)
	}

	if tenantLookups != 1 {
		t.Fatalf("expected 1 tenant lookup, got %d", tenantLookups)
	}
	if workspaceLookups != 1 {
		t.Fatalf("expected 1 workspace lookup, got %d", workspaceLookups)
	}
	if templateTypeLookups != 1 {
		t.Fatalf("expected 1 template type lookup, got %d", templateTypeLookups)
	}
	if templateLookups != 1 {
		t.Fatalf("expected 1 template lookup, got %d", templateLookups)
	}
	if publishedVersionLookups != 1 {
		t.Fatalf("expected 1 published version lookup, got %d", publishedVersionLookups)
	}
	if adapterLookups != 1 {
		t.Fatalf("expected 1 adapter lookup, got %d", adapterLookups)
	}
	if defaultIdentityLookups != 1 {
		t.Fatalf("expected 1 default identity lookup, got %d", defaultIdentityLookups)
	}
	if len(f.emailStore.emails) != 2 {
		t.Fatalf("expected 2 emails to be created, got %d", len(f.emailStore.emails))
	}
}

func TestSendService_SendBatch_AmortizesWorkspaceDefaultLocaleResolution(t *testing.T) {
	f := newSendFixture()

	var tenantLookups int
	var workspaceLookups int
	var templateTypeLookups int
	var templateLookups int
	var publishedVersionLookups int

	f.tenantStore.getByCodeFn = func(_ context.Context, code string) (*domain.Tenant, error) {
		tenantLookups++
		if code != "latam" {
			return nil, domain.ErrNotFound
		}
		return &domain.Tenant{ID: f.tenantID, Code: "latam", Name: "LATAM"}, nil
	}
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
		workspaceLookups++
		if tenantID != f.tenantID || code != "acme" {
			return nil, domain.ErrNotFound
		}
		defaultLocale := "en"
		return &domain.Workspace{
			ID:            f.workspaceID,
			TenantID:      f.tenantID,
			Code:          "acme",
			Name:          "Acme",
			Environment:   domain.EnvironmentProd,
			DefaultLocale: &defaultLocale,
		}, nil
	}
	f.templateStore.getTypeBySlugFn = func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
		templateTypeLookups++
		if slug != "welcome" {
			return nil, domain.ErrTemplateTypeNotFound
		}
		return &domain.TemplateType{
			ID:        f.typeID,
			Slug:      "welcome",
			Name:      "Welcome Email",
			AdapterID: &f.adapterID,
		}, nil
	}
	f.templateStore.resolveTemplateFn = func(_ context.Context, typeID uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
		templateLookups++
		if typeID != f.typeID {
			return nil, domain.ErrTemplateNotFound
		}
		return &domain.Template{ID: f.templateID, TemplateTypeID: f.typeID}, nil
	}
	f.templateStore.getPublishedVersionFn = func(_ context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
		publishedVersionLookups++
		if templateID != f.templateID {
			return nil, domain.ErrNoPublishedVersion
		}
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

	f.emailStore.createFn = func(_ context.Context, _ *domain.Email) error { return nil }
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "alice@user.com", Variables: map[string]any{"name": "Alice"}},
			{To: "bob@user.com", Variables: map[string]any{"name": "Bob"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %q", resp.Status)
	}

	if tenantLookups != 1 {
		t.Fatalf("expected 1 tenant lookup, got %d", tenantLookups)
	}
	if workspaceLookups != 1 {
		t.Fatalf("expected 1 workspace lookup, got %d", workspaceLookups)
	}
	if templateTypeLookups != 1 {
		t.Fatalf("expected default locale to reuse base template type lookup, got %d", templateTypeLookups)
	}
	if templateLookups != 1 {
		t.Fatalf("expected default locale to reuse base template lookup, got %d", templateLookups)
	}
	if publishedVersionLookups != 1 {
		t.Fatalf("expected default locale to reuse base published version lookup, got %d", publishedVersionLookups)
	}
}

func TestSendService_SendBatch_UsesWorkspaceDefaultLocaleSemantics(t *testing.T) {
	f := newSendFixture()

	var templateTypeLookups int
	var templateLookups int
	var publishedVersionLookups int
	var localeLookups int

	defaultLocale := "es"
	subjectES := "Asunto ES"
	fromNameES := "Equipo ES"
	bodyES := "<mj-text>Hola ES</mj-text>"

	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID != f.tenantID || code != "acme" {
			return nil, domain.ErrNotFound
		}
		return &domain.Workspace{
			ID:            f.workspaceID,
			TenantID:      f.tenantID,
			Code:          "acme",
			Name:          "Acme",
			Environment:   domain.EnvironmentProd,
			DefaultLocale: &defaultLocale,
		}, nil
	}
	f.templateStore.getTypeBySlugFn = func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
		templateTypeLookups++
		if slug != "welcome" {
			return nil, domain.ErrTemplateTypeNotFound
		}
		return &domain.TemplateType{
			ID:        f.typeID,
			Slug:      "welcome",
			Name:      "Welcome Email",
			AdapterID: &f.adapterID,
		}, nil
	}
	f.templateStore.resolveTemplateFn = func(_ context.Context, typeID uuid.UUID, _ []uuid.NullUUID) (*domain.Template, error) {
		templateLookups++
		if typeID != f.typeID {
			return nil, domain.ErrTemplateNotFound
		}
		return &domain.Template{
			ID:             f.templateID,
			TemplateTypeID: f.typeID,
		}, nil
	}
	f.templateStore.getPublishedVersionFn = func(_ context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
		publishedVersionLookups++
		if templateID != f.templateID {
			return nil, domain.ErrNoPublishedVersion
		}
		return &domain.TemplateVersion{
			ID:            f.versionID,
			TemplateID:    f.templateID,
			VersionNumber: 1,
			Status:        domain.VersionStatusPublished,
			Subject:       "Subject EN",
			PreviewText:   "Preview EN",
			FromName:      "Team EN",
			BodyMJML:      "<mj-text>Hello EN</mj-text>",
			DefaultLocale: "en",
		}, nil
	}
	f.templateStore.getLocaleFn = func(_ context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
		localeLookups++
		if versionID != f.versionID {
			return nil, domain.ErrNotFound
		}
		if locale != "es" {
			return nil, domain.ErrNotFound
		}
		return &domain.TemplateVersionLocale{
			ID:                uuid.Must(uuid.NewV7()),
			TemplateVersionID: f.versionID,
			Locale:            "es",
			Subject:           &subjectES,
			FromName:          &fromNameES,
			BodyMJML:          &bodyES,
		}, nil
	}

	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "alice@user.com", Variables: map[string]any{"name": "Alice"}},
			{To: "bob@user.com", Variables: map[string]any{"name": "Bob"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %q", resp.Status)
	}

	if localeLookups != 1 {
		t.Fatalf("expected 1 locale lookup for workspace default locale reuse, got %d", localeLookups)
	}
	if templateTypeLookups != 1 {
		t.Fatalf("expected 1 template type lookup, got %d", templateTypeLookups)
	}
	if templateLookups != 1 {
		t.Fatalf("expected 1 template lookup, got %d", templateLookups)
	}
	if publishedVersionLookups != 1 {
		t.Fatalf("expected 1 published version lookup, got %d", publishedVersionLookups)
	}

	if len(f.emailStore.emails) != 2 {
		t.Fatalf("expected 2 created emails, got %d", len(f.emailStore.emails))
	}
	for i, email := range f.emailStore.emails {
		if email.SubjectRendered != subjectES {
			t.Fatalf("email %d expected subject %q, got %q", i, subjectES, email.SubjectRendered)
		}
		if email.FromName != fromNameES {
			t.Fatalf("email %d expected from name %q, got %q", i, fromNameES, email.FromName)
		}
		if email.BodyMJML != bodyES {
			t.Fatalf("email %d expected body %q, got %q", i, bodyES, email.BodyMJML)
		}
	}
}

func TestSendService_SendBatch_PartialStatus(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		if email == "suppressed@user.com" {
			return true, "workspace", nil
		}
		return false, "", nil
	}
	f.emailStore.createFn = func(_ context.Context, email *domain.Email) error {
		if email.RecipientEmail == "failed@user.com" {
			return errors.New("db down")
		}
		return nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "accepted@user.com", Variables: map[string]any{"name": "Accepted"}},
			{To: "suppressed@user.com", Variables: map[string]any{"name": "Suppressed"}},
			{To: "failed@user.com", Variables: map[string]any{"name": "Failed"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "partial" {
		t.Fatalf("expected partial, got %q", resp.Status)
	}
	if resp.AcceptedCount != 1 || resp.SuppressedCount != 1 || resp.FailedCount != 1 {
		t.Fatalf("unexpected counters: %+v", resp)
	}
}

func TestSendService_SendBatch_MarksMixedFanoutItemAsPartial(t *testing.T) {
	f := newSendFixture()

	replaceMode := domain.TestRecipientModeReplace
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID != f.tenantID || code != "acme" {
			return nil, domain.ErrNotFound
		}
		return &domain.Workspace{
			ID:                     f.workspaceID,
			TenantID:               f.tenantID,
			Code:                   "acme",
			Name:                   "Acme Test",
			Environment:            domain.EnvironmentTest,
			TestRecipientMode:      replaceMode,
			TestRecipientAddresses: []string{"fanout-accepted@user.com", "fanout-failed@user.com"},
		}, nil
	}

	f.emailStore.createFn = func(_ context.Context, email *domain.Email) error {
		if email.RecipientEmail == "fanout-failed@user.com" {
			return errors.New("db down")
		}
		return nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "original@user.com", Variables: map[string]any{"name": "Accepted"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "partial" {
		t.Fatalf("expected partial batch status, got %q", resp.Status)
	}
	if resp.AcceptedCount != 0 || resp.SuppressedCount != 0 || resp.FailedCount != 1 {
		t.Fatalf("expected partial item to count as failed item, got accepted=%d suppressed=%d failed=%d", resp.AcceptedCount, resp.SuppressedCount, resp.FailedCount)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item result, got %d", len(resp.Items))
	}
	if resp.Items[0].Status != "partial" {
		t.Fatalf("expected mixed fan-out item status partial, got %q", resp.Items[0].Status)
	}
	if resp.Items[0].TrackingID == "" {
		t.Fatal("expected partial item to keep a tracking id from the accepted branch")
	}
}

func TestSendService_SendBatch_UsesSingleSetBasedSuppressionLookup(t *testing.T) {
	f := newSendFixture()

	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, _ string) (bool, string, error) {
		return false, "", errors.New("legacy per-address suppression path should not run")
	}
	f.suppression.getStatusesFn = func(_ context.Context, wsID uuid.UUID, emails []string) (map[string]port.SuppressionStatus, error) {
		if wsID != f.workspaceID {
			t.Fatalf("expected workspace %s, got %s", f.workspaceID, wsID)
		}

		got := append([]string(nil), emails...)
		slices.Sort(got)
		want := []string{
			"accepted@user.com",
			"blocked-bcc@user.com",
			"blocked-cc@user.com",
			"blocked-to@user.com",
			"shared@user.com",
			"visible-bcc@user.com",
		}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("expected unique suppression set %v, got %v", want, got)
		}

		return map[string]port.SuppressionStatus{
			"blocked-to@user.com":  {Suppressed: true, Reason: string(domain.SuppressionComplaint)},
			"blocked-cc@user.com":  {Suppressed: true, Reason: string(domain.SuppressionManual)},
			"blocked-bcc@user.com": {Suppressed: true, Reason: string(domain.SuppressionHardBounce)},
		}, nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{
				To:        "accepted@user.com",
				CC:        []string{"blocked-cc@user.com", "shared@user.com"},
				BCC:       []string{"blocked-bcc@user.com"},
				Variables: map[string]any{"name": "Accepted"},
			},
			{
				To:        "blocked-to@user.com",
				CC:        []string{"shared@user.com", "blocked-cc@user.com"},
				BCC:       []string{"visible-bcc@user.com"},
				Variables: map[string]any{"name": "Blocked"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.suppression.getStatusesCalls != 1 {
		t.Fatalf("expected exactly 1 set-based suppression lookup, got %d", f.suppression.getStatusesCalls)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted batch status, got %q", resp.Status)
	}
	if resp.AcceptedCount != 1 || resp.SuppressedCount != 1 || resp.FailedCount != 0 {
		t.Fatalf("unexpected counters: %+v", resp)
	}
	if len(f.emailStore.emails) != 2 {
		t.Fatalf("expected 2 persisted emails, got %d", len(f.emailStore.emails))
	}
	if got := f.emailStore.emails[0].CC; len(got) != 1 || got[0] != "shared@user.com" {
		t.Fatalf("expected accepted item CC to keep only visible recipients, got %v", got)
	}
	if got := f.emailStore.emails[0].BCC; len(got) != 0 {
		t.Fatalf("expected accepted item BCC to drop suppressed recipients, got %v", got)
	}
	if f.emailStore.emails[1].Status != domain.StatusSuppressed {
		t.Fatalf("expected second email status %q, got %q", domain.StatusSuppressed, f.emailStore.emails[1].Status)
	}
	if got := f.emailStore.emails[1].CC; len(got) != 1 || got[0] != "shared@user.com" {
		t.Fatalf("expected suppressed item CC to keep only visible recipients, got %v", got)
	}
	if got := f.emailStore.emails[1].BCC; len(got) != 1 || got[0] != "visible-bcc@user.com" {
		t.Fatalf("expected suppressed item BCC to keep only visible recipients, got %v", got)
	}
}

func TestSendService_SendBatch_AllFailed(t *testing.T) {
	f := newSendFixture()
	f.emailStore.createFn = func(_ context.Context, _ *domain.Email) error {
		return errors.New("db down")
	}

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "failed-1@user.com", Variables: map[string]any{"name": "One"}},
			{To: "failed-2@user.com", Variables: map[string]any{"name": "Two"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "failed" {
		t.Fatalf("expected failed, got %q", resp.Status)
	}
	if resp.FailedCount != 2 {
		t.Fatalf("expected 2 failed items, got %d", resp.FailedCount)
	}
}

func TestSendService_SendBatch_PreservesLocaleAndExternalIDPerItem(t *testing.T) {
	f := newSendFixture()

	var capturedEmails []*domain.Email
	f.emailStore.createFn = func(_ context.Context, email *domain.Email) error {
		capturedEmails = append(capturedEmails, email)
		return nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	localeA := "es"
	localeB := "en"
	externalA := "msg-a"
	externalB := "msg-b"

	_, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "alice@user.com", Locale: &localeA, ExternalID: &externalA, Variables: map[string]any{"name": "Alice"}},
			{To: "bob@user.com", Locale: &localeB, ExternalID: &externalB, Variables: map[string]any{"name": "Bob"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedEmails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(capturedEmails))
	}
	if capturedEmails[0].ExternalID == nil || *capturedEmails[0].ExternalID != externalA {
		t.Fatalf("expected first external id %q, got %+v", externalA, capturedEmails[0].ExternalID)
	}
	if capturedEmails[1].ExternalID == nil || *capturedEmails[1].ExternalID != externalB {
		t.Fatalf("expected second external id %q, got %+v", externalB, capturedEmails[1].ExternalID)
	}
	if capturedEmails[0].Locale == nil || *capturedEmails[0].Locale != localeA {
		t.Fatalf("expected first locale %q, got %+v", localeA, capturedEmails[0].Locale)
	}
	if capturedEmails[1].Locale == nil || *capturedEmails[1].Locale != localeB {
		t.Fatalf("expected second locale %q, got %+v", localeB, capturedEmails[1].Locale)
	}
}

func TestSendService_Send_DefaultsSourceTypeToAPIKey(t *testing.T) {
	f := newSendFixture()

	var captured *domain.Email
	f.emailStore.createFn = func(_ context.Context, email *domain.Email) error {
		captured = email
		return nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	_, err := svc.Send(context.Background(), f.happyRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured == nil {
		t.Fatal("expected email to be captured")
	}
	if captured.SourceType != domain.EmailSourceTypeDataPlaneAPIKey {
		t.Fatalf("expected source type %q, got %q", domain.EmailSourceTypeDataPlaneAPIKey, captured.SourceType)
	}
	if captured.SourceActorMemberID != nil {
		t.Fatalf("expected nil source actor member ID, got %v", *captured.SourceActorMemberID)
	}
	if captured.SourceActorEmail != nil {
		t.Fatalf("expected nil source actor email, got %q", *captured.SourceActorEmail)
	}
}

func TestSendService_SendBatch_PersistsUISourcePerItem(t *testing.T) {
	f := newSendFixture()

	var capturedEmails []*domain.Email
	f.emailStore.createFn = func(_ context.Context, email *domain.Email) error {
		capturedEmails = append(capturedEmails, email)
		return nil
	}
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	memberID := uuid.Must(uuid.NewV7())
	memberEmail := "editor@acme.com"

	svc := f.buildService()
	_, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "alice@user.com", Variables: map[string]any{"name": "Alice"}},
			{To: "bob@user.com", Variables: map[string]any{"name": "Bob"}},
		},
		Source: service.SendSource{
			Type:          domain.EmailSourceTypeManagementTemplateBulkUpload,
			ActorMemberID: &memberID,
			ActorEmail:    &memberEmail,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedEmails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(capturedEmails))
	}

	for i, email := range capturedEmails {
		if email.SourceType != domain.EmailSourceTypeManagementTemplateBulkUpload {
			t.Fatalf("email %d source type = %q, want %q", i, email.SourceType, domain.EmailSourceTypeManagementTemplateBulkUpload)
		}
		if email.SourceActorMemberID == nil || *email.SourceActorMemberID != memberID {
			t.Fatalf("email %d actor member = %+v, want %s", i, email.SourceActorMemberID, memberID)
		}
		if email.SourceActorEmail == nil || *email.SourceActorEmail != memberEmail {
			t.Fatalf("email %d actor email = %+v, want %q", i, email.SourceActorEmail, memberEmail)
		}
	}
}

// Budget targets:
// - SendBatch amortized path should keep shared resolution sublinear as item count grows.
// - The batch path should beat the legacy item-by-item Send loop on allocs/op for identical input.
func BenchmarkSendService_SendBatch_Amortized(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name  string
		items int
	}{
		{name: "batch_1_item", items: 1},
		{name: "batch_10_items", items: 10},
	}

	for _, tt := range cases {
		b.Run(tt.name, func(b *testing.B) {
			b.Run("amortized", func(b *testing.B) {
				runSendBatchAmortizedBenchmark(b, tt.items)
			})

			b.Run("legacy_item_by_item", func(b *testing.B) {
				runSendLegacyBenchmark(b, tt.items)
			})
		})
	}
}

func runSendBatchAmortizedBenchmark(b *testing.B, items int) {
	b.StopTimer()
	f, svc := newBenchmarkSendService()
	batchReq := benchmarkBatchRequest(items)
	b.ResetTimer()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		resetBenchmarkSendFixture(f)
		resp, err := svc.SendBatch(context.Background(), batchReq)
		if err != nil {
			b.Fatalf("SendBatch() error: %v", err)
		}
		sinkSendBatchResponse = resp
	}
}

func runSendLegacyBenchmark(b *testing.B, items int) {
	b.StopTimer()
	f, svc := newBenchmarkSendService()
	itemReqs := benchmarkLegacyItemRequests(items)
	b.ResetTimer()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		resetBenchmarkSendFixture(f)
		for _, req := range itemReqs {
			resp, err := svc.Send(context.Background(), req)
			if err != nil {
				b.Fatalf("Send() error: %v", err)
			}
			sinkSendBatchResponse = &service.SendBatchResponse{
				Status:           resp.Status,
				TemplateResolved: resp.TemplateResolved,
			}
		}
	}
}

func newBenchmarkSendService() (*sendTestFixture, *service.SendService) {
	f := newSendFixture()
	f.emailStore.createFn = func(_ context.Context, _ *domain.Email) error { return nil }
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }
	return f, f.buildService()
}

func benchmarkBatchRequest(items int) *service.SendBatchRequest {
	req := &service.SendBatchRequest{
		Ref:   "latam:acme:welcome",
		Items: make([]service.SendBatchItemRequest, items),
	}
	for i := 0; i < items; i++ {
		req.Items[i] = service.SendBatchItemRequest{
			To:        benchmarkRecipient(i),
			Variables: map[string]any{"name": "Alice"},
		}
	}
	return req
}

func benchmarkLegacyItemRequests(items int) []*service.SendRequest {
	reqs := make([]*service.SendRequest, items)
	for i := 0; i < items; i++ {
		reqs[i] = &service.SendRequest{
			Ref:       "latam:acme:welcome",
			To:        []string{benchmarkRecipient(i)},
			Variables: map[string]any{"name": "Alice"},
		}
	}
	return reqs
}

func benchmarkRecipient(i int) string {
	if i%2 == 1 {
		return "other@example.com"
	}
	return "user@example.com"
}

func resetBenchmarkSendFixture(f *sendTestFixture) {
	f.cache.data = make(map[string][]byte)
	f.emailStore.emails = f.emailStore.emails[:0]
}

func BenchmarkResolvedSendContext_DefaultLocaleReuse(b *testing.B) {
	b.ReportAllocs()

	f := newSendFixture()
	f.wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID != f.tenantID || code != "acme" {
			return nil, domain.ErrNotFound
		}
		defaultLocale := "en"
		return &domain.Workspace{
			ID:            f.workspaceID,
			TenantID:      f.tenantID,
			Code:          "acme",
			Name:          "Acme",
			Environment:   domain.EnvironmentProd,
			DefaultLocale: &defaultLocale,
		}, nil
	}
	f.emailStore.createFn = func(_ context.Context, _ *domain.Email) error { return nil }
	f.jq.enqueueSendFn = func(_ context.Context, _ *port.SendJob) error { return nil }

	svc := f.buildService()
	req := &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{
			{To: "alice@user.com", Variables: map[string]any{"name": "Alice"}},
			{To: "bob@user.com", Variables: map[string]any{"name": "Bob"}},
			{To: "carol@user.com", Variables: map[string]any{"name": "Carol"}},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.cache.data = make(map[string][]byte)
		f.emailStore.emails = f.emailStore.emails[:0]
		resp, err := svc.SendBatch(context.Background(), req)
		if err != nil {
			b.Fatalf("SendBatch() error: %v", err)
		}
		sinkSendBatchResponse = resp
	}
}

type stubCodeInjectorSend struct {
	code      string
	resolveFn port.CodeResolveFunc
}

var sinkSendBatchResponse *service.SendBatchResponse

func (s *stubCodeInjectorSend) Code() string { return s.code }
func (s *stubCodeInjectorSend) Resolve() (port.CodeResolveFunc, []string) {
	return s.resolveFn, nil
}
func (s *stubCodeInjectorSend) IsCritical() bool       { return false }
func (s *stubCodeInjectorSend) Timeout() time.Duration { return 0 }

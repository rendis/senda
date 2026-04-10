package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	mjmladapter "github.com/rendis/senda/internal/adapter/mjml"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// --- Helpers ---

type mockResolvedTemplateInvalidator struct {
	invalidateResolvedTemplatesFn func(ctx context.Context, workspaceID uuid.UUID)
	invalidateAllFn               func(ctx context.Context)
}

func (m *mockResolvedTemplateInvalidator) InvalidateResolvedTemplates(ctx context.Context, workspaceID uuid.UUID) {
	if m.invalidateResolvedTemplatesFn != nil {
		m.invalidateResolvedTemplatesFn(ctx, workspaceID)
	}
}

func (m *mockResolvedTemplateInvalidator) InvalidateAllResolvedTemplates(ctx context.Context) {
	if m.invalidateAllFn != nil {
		m.invalidateAllFn(ctx)
	}
}

func setupTemplateTest(store port.TemplateStore, compiler port.TemplateCompiler, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.TemplateHandler) {
	return setupTemplateTestWithOptions(store, compiler, ts, ws, nil, nil, nil, 100)
}

func setupTemplateTestWithInvalidator(
	store port.TemplateStore,
	compiler port.TemplateCompiler,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	invalidator *mockResolvedTemplateInvalidator,
) (*echo.Echo, *handler.TemplateHandler) {
	return setupTemplateTestWithOptions(store, compiler, ts, ws, nil, invalidator, nil, 100)
}

type fakeTemplateBatchSender struct {
	sendBatchFn func(ctx context.Context, req *service.SendBatchRequest) (*service.SendBatchResponse, error)
}

func (f *fakeTemplateBatchSender) Send(_ context.Context, _ *service.SendRequest) (*service.SendResponse, error) {
	return nil, nil
}

func (f *fakeTemplateBatchSender) SendBatch(ctx context.Context, req *service.SendBatchRequest) (*service.SendBatchResponse, error) {
	if f.sendBatchFn != nil {
		return f.sendBatchFn(ctx, req)
	}
	return &service.SendBatchResponse{}, nil
}

type mockAuditLogStoreTemplate struct {
	appendFn func(ctx context.Context, entry *domain.AuditLog) error
	entries  []*domain.AuditLog
}

func (m *mockAuditLogStoreTemplate) Append(ctx context.Context, entry *domain.AuditLog) error {
	m.entries = append(m.entries, entry)
	if m.appendFn != nil {
		return m.appendFn(ctx, entry)
	}
	return nil
}

func (m *mockAuditLogStoreTemplate) Query(_ context.Context, _ port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
	return &port.PageResult[domain.AuditLog]{}, nil
}

func setupTemplateTestWithOptions(
	store port.TemplateStore,
	compiler port.TemplateCompiler,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	batchSender *fakeTemplateBatchSender,
	invalidator *mockResolvedTemplateInvalidator,
	auditStore port.AuditLogStore,
	batchMaxItems int,
) (*echo.Echo, *handler.TemplateHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	svc := service.NewTemplateService(store, compiler)
	h := handler.NewTemplateHandler(svc, store, ts, ws, nil, batchSender, auditStore, batchMaxItems, invalidator)

	base := "/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code"

	e.POST(base+"/templates", h.CreateTemplate)
	e.GET(base+"/template-types/:slug/templates", h.ListByTemplateType)
	e.POST(base+"/templates/:template_id/fork", h.ForkTemplate)
	e.POST("/api/v1/manage/global/templates", h.CreateTemplateGlobal)
	e.GET(base+"/templates/:template_id/versions", h.ListVersions)
	e.POST(base+"/templates/:template_id/versions", h.CreateVersion)
	e.PUT(base+"/templates/:template_id/versions/:version_id", h.UpdateVersion)
	e.POST(base+"/templates/:template_id/versions/:version_id/clone", h.CloneVersion)
	e.POST(base+"/templates/:template_id/versions/:version_id/publish", h.PublishVersion)
	e.POST(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.SetLocale)
	e.GET(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.GetLocale)
	e.PUT(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.UpdateLocale)
	e.DELETE(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.DeleteLocale)
	e.POST(base+"/templates/:template_id/preview-mjml", h.PreviewMJML)
	e.POST(base+"/templates/:template_id/test-send", h.TestSend)
	e.GET(base+"/templates/:template_id/bulk-send-config", h.BulkSendConfig)
	e.POST(base+"/templates/:template_id/bulk-send", h.BulkSend)
	e.POST(base+"/templates/:template_id/disable", h.DisableTemplate)
	e.POST(base+"/templates/:template_id/enable", h.EnableTemplate)
	e.POST("/api/v1/manage/global/templates/:template_id/disable", h.DisableTemplateGlobal)
	e.POST("/api/v1/manage/global/templates/:template_id/enable", h.EnableTemplateGlobal)

	return e, h
}

func workspaceOwnedTemplate(templateID, workspaceID uuid.UUID) *domain.Template {
	return &domain.Template{
		ID:             templateID,
		TemplateTypeID: uuid.Must(uuid.NewV7()),
		WorkspaceID:    &workspaceID,
	}
}

// --- Tests ---

func TestTemplateHandler_CreateTemplate_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var created *domain.Template
	store := &mockTemplateStore{
		createTemplateFn: func(_ context.Context, tpl *domain.Template) error {
			created = tpl
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	typeID := uuid.Must(uuid.NewV7())
	body := `{"template_type_id":"` + typeID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected template to be created")
	}
	if created.TemplateTypeID != typeID {
		t.Fatalf("expected template type ID %s, got %s", typeID, created.TemplateTypeID)
	}
	if created.WorkspaceID == nil || *created.WorkspaceID != ws.ID {
		t.Fatalf("expected workspace ID %s", ws.ID)
	}
}

func TestTemplateHandler_CreateTemplateGlobal_Success(t *testing.T) {
	var created *domain.Template
	store := &mockTemplateStore{
		createTemplateFn: func(_ context.Context, tpl *domain.Template) error {
			created = tpl
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, &mockTenantStore{}, &mockWorkspaceStore{})

	typeID := uuid.Must(uuid.NewV7())
	body := `{"template_type_id":"` + typeID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/global/templates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected template to be created")
	}
	if created.WorkspaceID != nil {
		t.Fatal("expected nil workspace ID for global template")
	}
}

func TestTemplateHandler_CreateTemplate_MissingTypeID(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_DisableTemplate_InvalidatesResolvedTemplateCache(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	invalidatedWorkspace := uuid.Nil

	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		setDisabledFn: func(_ context.Context, gotTemplateID uuid.UUID, gotWSID *uuid.UUID, disabled bool) error {
			if gotTemplateID != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, gotTemplateID)
			}
			if gotWSID == nil || *gotWSID != ws.ID {
				t.Fatalf("expected workspace ID %s", ws.ID)
			}
			if !disabled {
				t.Fatal("expected disable=true")
			}
			return nil
		},
	}
	invalidator := &mockResolvedTemplateInvalidator{
		invalidateResolvedTemplatesFn: func(_ context.Context, workspaceID uuid.UUID) {
			invalidatedWorkspace = workspaceID
		},
	}

	e, _ := setupTemplateTestWithInvalidator(store, &mockTemplateCompiler{}, ts, wsStore, invalidator)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/disable", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if invalidatedWorkspace != ws.ID {
		t.Fatalf("expected invalidated workspace %s, got %s", ws.ID, invalidatedWorkspace)
	}
}

func TestTemplateHandler_CreateTemplate_InvalidTypeID(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	body := `{"template_type_id":"not-a-uuid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_CreateVersion_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	var created *domain.TemplateVersion
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		createVersionFn: func(_ context.Context, ver *domain.TemplateVersion) error {
			ver.VersionNumber = 1
			created = ver
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	body := `{"subject":"Welcome","preview_text":"Hello!","from_name":"Acme","from_email":"noreply@acme.com","body_mjml":"<mjml></mjml>","default_locale":"en"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected version to be created")
	}
	if created.Status != domain.VersionStatusDraft {
		t.Fatalf("expected status 'draft', got %q", created.Status)
	}
	if created.VersionNumber != 1 {
		t.Fatalf("expected version number 1, got %d", created.VersionNumber)
	}
	if created.Subject != "Welcome" {
		t.Fatalf("expected subject 'Welcome', got %q", created.Subject)
	}
}

func TestTemplateHandler_CreateVersion_MissingRequired(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	e, _ := setupTemplateTest(&mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
	}, &mockTemplateCompiler{}, ts, wsStore)

	body := `{"preview_text":"Preview only"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_CreateVersion_InvalidTemplateID(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	body := `{"subject":"Test","from_name":"A","from_email":"a@b.com","body_mjml":"<mjml></mjml>","default_locale":"en"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/not-a-uuid/versions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_ListVersions_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	store := &mockTemplateStore{
		listVersionsFn: func(_ context.Context, id uuid.UUID) ([]*domain.TemplateVersion, error) {
			if id == templateID {
				return []*domain.TemplateVersion{
					{ID: uuid.Must(uuid.NewV7()), TemplateID: templateID, VersionNumber: 1, Status: domain.VersionStatusPublished, Subject: "V1", CreatedAt: now, UpdatedAt: now},
					{ID: uuid.Must(uuid.NewV7()), TemplateID: templateID, VersionNumber: 2, Status: domain.VersionStatusDraft, Subject: "V2", CreatedAt: now, UpdatedAt: now},
				}, nil
			}
			return nil, nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateVersionListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
}

func TestTemplateHandler_ListByTemplateType_ResolvesVisibleSystemTemplateInWorkspace(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemWorkspace := uuid.Must(uuid.NewV7())
	typeID := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())

	wsStore.getSystemWorkspaceFn = func(_ context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			t.Fatalf("expected tenant %s, got %s", tenant.ID, tenantID)
		}
		return &domain.Workspace{
			ID:       systemWorkspace,
			TenantID: tenant.ID,
			Code:     "_system",
			Name:     "Default",
			IsSystem: true,
		}, nil
	}

	store := &mockTemplateStore{
		getTypeBySlugFn: func(_ context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error) {
			if slug != "welcome-email" {
				t.Fatalf("unexpected slug %q", slug)
			}
			if len(chain) != 2 {
				t.Fatalf("expected 2 scopes, got %d", len(chain))
			}
			return &domain.TemplateType{
				ID:          typeID,
				WorkspaceID: &systemWorkspace,
				Slug:        "welcome-email",
				Name:        "Welcome",
			}, nil
		},
		resolveTemplateFn: func(_ context.Context, gotTypeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error) {
			if gotTypeID != typeID {
				t.Fatalf("expected type id %s, got %s", typeID, gotTypeID)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: typeID,
				WorkspaceID:    &systemWorkspace,
			}, nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/template-types/welcome-email/templates", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 visible template, got %d", len(resp.Items))
	}
	if !resp.Items[0].InheritedFromSystem {
		t.Fatal("expected template to be marked as inherited from system")
	}
	if resp.Items[0].OwnerScope != "system" {
		t.Fatalf("expected owner_scope system, got %q", resp.Items[0].OwnerScope)
	}
}

func TestTemplateHandler_CreateVersion_BlocksInheritedSystemTemplateInWorkspace(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemWorkspace := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())

	wsStore.getSystemWorkspaceFn = func(_ context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			t.Fatalf("expected tenant %s, got %s", tenant.ID, tenantID)
		}
		return &domain.Workspace{
			ID:       systemWorkspace,
			TenantID: tenant.ID,
			Code:     "_system",
			Name:     "Default",
			IsSystem: true,
		}, nil
	}

	createCalled := false
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template id %s, got %s", templateID, id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: uuid.Must(uuid.NewV7()),
				WorkspaceID:    &systemWorkspace,
			}, nil
		},
		createVersionFn: func(_ context.Context, _ *domain.TemplateVersion) error {
			createCalled = true
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	body := `{"subject":"Welcome","from_name":"Acme","body_mjml":"<mjml></mjml>","default_locale":"en"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if createCalled {
		t.Fatal("expected inherited template version creation to be blocked")
	}
}

func TestTemplateHandler_UpdateVersion_BlocksInheritedSystemTemplateInWorkspace(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemWorkspace := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())
	versionID := uuid.Must(uuid.NewV7())

	wsStore.getSystemWorkspaceFn = func(_ context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			t.Fatalf("expected tenant %s, got %s", tenant.ID, tenantID)
		}
		return &domain.Workspace{
			ID:       systemWorkspace,
			TenantID: tenant.ID,
			Code:     "_system",
			Name:     "Default",
			IsSystem: true,
		}, nil
	}

	updateCalled := false
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template id %s, got %s", templateID, id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: uuid.Must(uuid.NewV7()),
				WorkspaceID:    &systemWorkspace,
			}, nil
		},
		getVersionByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateVersion, error) {
			if id != versionID {
				t.Fatalf("expected version id %s, got %s", versionID, id)
			}
			return &domain.TemplateVersion{
				ID:         versionID,
				TemplateID: templateID,
				Status:     domain.VersionStatusDraft,
			}, nil
		},
		updateVersionFn: func(_ context.Context, _ *domain.TemplateVersion) error {
			updateCalled = true
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	body := `{"subject":"Updated subject"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if updateCalled {
		t.Fatal("expected inherited template version updates to be blocked")
	}
}

func TestTemplateHandler_ForkTemplate_Success(t *testing.T) {
	tenant, ws, ts, wsStore := testTenantAndWorkspace()
	templateID := uuid.Must(uuid.NewV7())
	forkedTemplateID := uuid.Must(uuid.NewV7())
	systemWorkspace := uuid.Must(uuid.NewV7())
	invalidatedWorkspace := uuid.Nil

	wsStore.getSystemWorkspaceFn = func(_ context.Context, tenantID uuid.UUID) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			t.Fatalf("expected tenant %s, got %s", tenant.ID, tenantID)
		}
		return &domain.Workspace{
			ID:       systemWorkspace,
			TenantID: tenant.ID,
			Code:     "_system",
			Name:     "Default",
			IsSystem: true,
		}, nil
	}

	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template %s, got %s", templateID, id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: uuid.Must(uuid.NewV7()),
				WorkspaceID:    &systemWorkspace,
			}, nil
		},
		forkTemplateFn: func(_ context.Context, sourceTemplateID uuid.UUID, workspaceID uuid.UUID, createdBy *uuid.UUID) (*domain.Template, error) {
			if sourceTemplateID != templateID {
				t.Fatalf("expected source template %s, got %s", templateID, sourceTemplateID)
			}
			if workspaceID != ws.ID {
				t.Fatalf("expected workspace %s, got %s", ws.ID, workspaceID)
			}
			if createdBy != nil {
				t.Fatalf("expected nil createdBy in handler test, got %s", *createdBy)
			}
			return &domain.Template{
				ID:               forkedTemplateID,
				TemplateTypeID:   uuid.Must(uuid.NewV7()),
				WorkspaceID:      &workspaceID,
				IsFork:           true,
				OriginTemplateID: &sourceTemplateID,
			}, nil
		},
	}

	e, _ := setupTemplateTestWithInvalidator(store, &mockTemplateCompiler{}, ts, wsStore, &mockResolvedTemplateInvalidator{
		invalidateResolvedTemplatesFn: func(_ context.Context, workspaceID uuid.UUID) {
			invalidatedWorkspace = workspaceID
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/fork", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.IsFork {
		t.Fatal("expected response template to be marked as fork")
	}
	if resp.OriginTemplateID == nil || *resp.OriginTemplateID != templateID.String() {
		t.Fatalf("expected origin template %s, got %#v", templateID, resp.OriginTemplateID)
	}
	if invalidatedWorkspace != ws.ID {
		t.Fatalf("expected resolved template cache invalidation for workspace %s, got %s", ws.ID, invalidatedWorkspace)
	}
}

func TestTemplateHandler_PublishVersion_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	versionID := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())
	var publishedID uuid.UUID
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		publishFn: func(_ context.Context, id uuid.UUID) error {
			publishedID = id
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/publish", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if publishedID != versionID {
		t.Fatalf("expected published version ID %s, got %s", versionID, publishedID)
	}
}

func TestTemplateHandler_PublishVersion_InvalidVersionID(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	e, _ := setupTemplateTest(&mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
	}, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/bad-id/publish", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_PublishVersion_NotFound(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		publishFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrNotFound
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	versionID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/publish", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_CloneVersion_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	sourceVersionID := uuid.Must(uuid.NewV7())
	clonedVersionID := uuid.Must(uuid.NewV7())

	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return &domain.Template{ID: templateID, WorkspaceID: &ws.ID}, nil
		},
		getVersionByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateVersion, error) {
			if id != sourceVersionID {
				t.Fatalf("expected source version ID %s, got %s", sourceVersionID, id)
			}
			return &domain.TemplateVersion{ID: sourceVersionID, TemplateID: templateID, Status: domain.VersionStatusArchived}, nil
		},
		cloneVersionFn: func(_ context.Context, gotTemplateID, gotSourceVersionID uuid.UUID, _ *uuid.UUID) (*domain.TemplateVersion, error) {
			if gotTemplateID != templateID {
				t.Fatalf("expected clone template ID %s, got %s", templateID, gotTemplateID)
			}
			if gotSourceVersionID != sourceVersionID {
				t.Fatalf("expected clone source version ID %s, got %s", sourceVersionID, gotSourceVersionID)
			}
			return &domain.TemplateVersion{
				ID:            clonedVersionID,
				TemplateID:    templateID,
				VersionNumber: 4,
				Status:        domain.VersionStatusDraft,
				Subject:       "Cloned version",
				CreatedAt:     time.Now().UTC(),
				UpdatedAt:     time.Now().UTC(),
			}, nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+sourceVersionID.String()+"/clone", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateVersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.ID != clonedVersionID.String() {
		t.Fatalf("expected cloned version ID %s, got %s", clonedVersionID, resp.ID)
	}
	if resp.Status != string(domain.VersionStatusDraft) {
		t.Fatalf("expected draft response, got %s", resp.Status)
	}
}

func TestTemplateHandler_CloneVersion_WorkspaceMismatch(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	sourceVersionID := uuid.Must(uuid.NewV7())
	otherWorkspaceID := uuid.Must(uuid.NewV7())

	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Template, error) {
			return &domain.Template{ID: templateID, WorkspaceID: &otherWorkspaceID}, nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+sourceVersionID.String()+"/clone", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_DisableTemplate_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	var (
		gotTemplateID uuid.UUID
		gotScope      *uuid.UUID
		gotDisabled   bool
	)

	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		setDisabledFn: func(_ context.Context, id uuid.UUID, scope *uuid.UUID, disabled bool) error {
			gotTemplateID = id
			gotScope = scope
			gotDisabled = disabled
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/disable", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotTemplateID != templateID {
		t.Fatalf("expected template ID %s, got %s", templateID, gotTemplateID)
	}
	if gotScope == nil || *gotScope != ws.ID {
		t.Fatalf("expected workspace scope %s, got %v", ws.ID, gotScope)
	}
	if !gotDisabled {
		t.Fatal("expected disabled=true")
	}
}

func TestTemplateHandler_DisableTemplate_WithoutInvalidator(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	var (
		gotTemplateID uuid.UUID
		gotScope      *uuid.UUID
		gotDisabled   bool
	)

	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		setDisabledFn: func(_ context.Context, id uuid.UUID, scope *uuid.UUID, disabled bool) error {
			gotTemplateID = id
			gotScope = scope
			gotDisabled = disabled
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/disable", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotTemplateID != templateID {
		t.Fatalf("expected template ID %s, got %s", templateID, gotTemplateID)
	}
	if gotScope == nil || *gotScope != ws.ID {
		t.Fatalf("expected workspace scope %s, got %v", ws.ID, gotScope)
	}
	if !gotDisabled {
		t.Fatal("expected disabled=true")
	}
}

func TestTemplateHandler_EnableTemplate_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	var gotDisabled bool
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		setDisabledFn: func(_ context.Context, _ uuid.UUID, _ *uuid.UUID, disabled bool) error {
			gotDisabled = disabled
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/enable", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotDisabled {
		t.Fatal("expected disabled=false")
	}
}

func TestTemplateHandler_DisableTemplateGlobal_Success(t *testing.T) {
	templateID := uuid.Must(uuid.NewV7())
	var gotScope *uuid.UUID
	store := &mockTemplateStore{
		setDisabledFn: func(_ context.Context, _ uuid.UUID, scope *uuid.UUID, _ bool) error {
			gotScope = scope
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/global/templates/"+templateID.String()+"/disable", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotScope != nil {
		t.Fatalf("expected nil global scope, got %v", gotScope)
	}
}

func TestTemplateHandler_SetLocale_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var set *domain.TemplateVersionLocale
	templateID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		setLocaleFn: func(_ context.Context, loc *domain.TemplateVersionLocale) error {
			set = loc
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	versionID := uuid.Must(uuid.NewV7())
	body := `{"subject":"Bienvenido","preview_text":"Hola!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/locales/es", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if set == nil {
		t.Fatal("expected locale to be set")
	}
	if set.Locale != "es" {
		t.Fatalf("expected locale 'es', got %q", set.Locale)
	}
	if set.Subject == nil || *set.Subject != "Bienvenido" {
		t.Fatal("expected subject 'Bienvenido'")
	}
}

func TestTemplateHandler_UpdateLocale_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		setLocaleFn: func(_ context.Context, _ *domain.TemplateVersionLocale) error {
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	versionID := uuid.Must(uuid.NewV7())
	body := `{"subject":"Updated Subject"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/locales/fr", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_GetLocale_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	versionID := uuid.Must(uuid.NewV7())
	subject := "Bonjour"
	now := time.Now().UTC()
	store := &mockTemplateStore{
		getLocaleFn: func(_ context.Context, vid uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
			if vid == versionID && locale == "fr" {
				return &domain.TemplateVersionLocale{
					ID:                uuid.Must(uuid.NewV7()),
					TemplateVersionID: versionID,
					Locale:            "fr",
					Subject:           &subject,
					CreatedAt:         now,
					UpdatedAt:         now,
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/locales/fr", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateVersionLocaleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Locale != "fr" {
		t.Fatalf("expected locale 'fr', got %q", resp.Locale)
	}
}

func TestTemplateHandler_GetLocale_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{
		getLocaleFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.TemplateVersionLocale, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	versionID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/locales/zh", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_DeleteLocale_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	e, _ := setupTemplateTest(&mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
	}, &mockTemplateCompiler{}, ts, wsStore)

	versionID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/locales/es", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_DeleteLocale_NotFound(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
		deleteLocaleFn: func(_ context.Context, _ uuid.UUID, _ string) error {
			return domain.ErrNotFound
		},
	}
	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	versionID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/locales/zh", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_PreviewMJML_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	compiler := &mockTemplateCompiler{
		compileFn: func(_ context.Context, mjml string) (string, error) {
			return "<html><body>" + mjml + "</body></html>", nil
		},
	}

	e, _ := setupTemplateTest(&mockTemplateStore{}, compiler, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	body := `{"mjml":"<mjml><mj-body></mj-body></mjml>"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/preview-mjml", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.MJMLPreviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.HTML == "" {
		t.Fatal("expected non-empty HTML")
	}
}

func TestTemplateHandler_PreviewMJML_RewritesVideoThumbnailUsingRequestBaseURL(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, mjmladapter.NewCompiler(), ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	body := `{"mjml":"<mjml><mj-body><mj-section><mj-column><mj-image src=\"https://img.youtube.com/vi/dQw4w9WgXcQ/maxresdefault.jpg\" href=\"https://www.youtube.com/watch?v=dQw4w9WgXcQ\" css-class=\"senda-video\" /></mj-column></mj-section></mj-body></mjml>"}`
	req := httptest.NewRequest(http.MethodPost, "http://builder.local/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/preview-mjml", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.MJMLPreviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !strings.Contains(resp.HTML, "http://builder.local/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2FdQw4w9WgXcQ%2Fmaxresdefault.jpg") {
		t.Fatalf("expected request-scoped video thumbnail URL, got %q", resp.HTML)
	}
}

func TestTemplateHandler_PreviewMJML_MissingMJML(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/preview-mjml", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_PreviewMJML_CompilerError(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	compiler := &mockTemplateCompiler{
		compileFn: func(_ context.Context, _ string) (string, error) {
			return "", domain.ErrValidation
		},
	}

	e, _ := setupTemplateTest(&mockTemplateStore{}, compiler, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	body := `{"mjml":"invalid mjml"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/preview-mjml", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_SetLocale_InvalidVersionID(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	e, _ := setupTemplateTest(&mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("expected template ID %s, got %s", templateID, id)
			}
			return workspaceOwnedTemplate(templateID, ws.ID), nil
		},
	}, &mockTemplateCompiler{}, ts, wsStore)

	body := `{"subject":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/not-valid/locales/en", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_BulkSendConfig_ReturnsConfiguredLimit(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTestWithOptions(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore, nil, nil, nil, 77)

	templateID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/bulk-send-config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		MaxItems        int    `json:"max_items"`
		VersionStrategy string `json:"version_strategy"`
		RequestShape    string `json:"request_shape"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.MaxItems != 77 {
		t.Fatalf("expected max_items 77, got %d", body.MaxItems)
	}
	if body.VersionStrategy != "published" {
		t.Fatalf("expected version strategy published, got %q", body.VersionStrategy)
	}
	if body.RequestShape != "items_only" {
		t.Fatalf("expected request shape items_only, got %q", body.RequestShape)
	}
}

func TestTemplateHandler_BulkSend_Validation(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTestWithOptions(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore, nil, nil, nil, 2)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("member", &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "editor@acme.com"})
			return next(c)
		}
	})

	templateID := uuid.Must(uuid.NewV7())
	body := `{"items":[{},{"to":"one@example.com"},{"to":"two@example.com"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/bulk-send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_BulkSend_SendsBatchAndWritesAudit(t *testing.T) {
	tenant, ws, ts, wsStore := testTenantAndWorkspace()
	templateID := uuid.Must(uuid.NewV7())
	templateTypeID := uuid.Must(uuid.NewV7())
	memberID := uuid.Must(uuid.NewV7())
	memberEmail := "editor@acme.com"

	store := &mockTemplateStore{
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: templateTypeID,
				WorkspaceID:    &ws.ID,
			}, nil
		},
		getTypeByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateType, error) {
			if id != templateTypeID {
				t.Fatalf("unexpected template type id %s", id)
			}
			return &domain.TemplateType{
				ID:   templateTypeID,
				Slug: "welcome",
			}, nil
		},
	}

	batchSender := &fakeTemplateBatchSender{
		sendBatchFn: func(_ context.Context, req *service.SendBatchRequest) (*service.SendBatchResponse, error) {
			if req.Ref != tenant.Code+":"+ws.Code+":welcome" {
				t.Fatalf("unexpected ref %q", req.Ref)
			}
			if req.Source.Type != domain.EmailSourceTypeManagementTemplateBulkUpload {
				t.Fatalf("unexpected source type %q", req.Source.Type)
			}
			if req.Source.ActorMemberID == nil || *req.Source.ActorMemberID != memberID {
				t.Fatalf("unexpected actor member id %+v", req.Source.ActorMemberID)
			}
			if req.Source.ActorEmail == nil || *req.Source.ActorEmail != memberEmail {
				t.Fatalf("unexpected actor email %+v", req.Source.ActorEmail)
			}
			if len(req.Items) != 2 {
				t.Fatalf("expected 2 items, got %d", len(req.Items))
			}
			if got := req.Items[0].Injectors["student"]["name"]; got != "Ana Override" {
				t.Fatalf("expected first item injector override, got %#v", req.Items[0].Injectors)
			}
			if got := req.Items[1].Injectors["student"]["name"]; got != "Beto Override" {
				t.Fatalf("expected second item injector override, got %#v", req.Items[1].Injectors)
			}
			return &service.SendBatchResponse{
				Status:           "partial",
				TemplateResolved: req.Ref,
				AcceptedCount:    1,
				FailedCount:      1,
				Items: []service.SendBatchItemResult{
					{Index: 0, To: "ana@example.com", TrackingID: "trk_1", Status: "accepted"},
					{Index: 1, To: "beto@example.com", Status: "failed", Error: "db down"},
				},
			}, nil
		},
	}
	auditStore := &mockAuditLogStoreTemplate{}

	e, _ := setupTemplateTestWithOptions(store, &mockTemplateCompiler{}, ts, wsStore, batchSender, nil, auditStore, 100)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("member", &domain.Member{ID: memberID, Email: memberEmail})
			return next(c)
		}
	})

	body := `{"items":[{"to":"ana@example.com","variables":{"name":"Ana"},"injectors":{"student":{"name":"Ana Override"}}},{"to":"beto@example.com","external_id":"msg-2","injectors":{"student":{"name":"Beto Override"}}}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/bulk-send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(auditStore.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditStore.entries))
	}
	entry := auditStore.entries[0]
	if entry.ActorID != memberID {
		t.Fatalf("expected audit actor id %s, got %s", memberID, entry.ActorID)
	}
	if entry.ActorEmail != memberEmail {
		t.Fatalf("expected audit actor email %q, got %q", memberEmail, entry.ActorEmail)
	}
	if entry.Action != domain.AuditBulkSend {
		t.Fatalf("expected audit action %q, got %q", domain.AuditBulkSend, entry.Action)
	}
	if entry.EntityType != "template" || entry.EntityID != templateID {
		t.Fatalf("unexpected audit entity %+v", entry)
	}
	if got := entry.Changes["accepted_count"]; got != 1 {
		t.Fatalf("expected accepted_count 1, got %#v", got)
	}
	if got := entry.Changes["failed_count"]; got != 1 {
		t.Fatalf("expected failed_count 1, got %#v", got)
	}
}

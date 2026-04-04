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
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
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
	deleteLocaleFn          func(ctx context.Context, versionID uuid.UUID, locale string) error
	listTypesFn             func(ctx context.Context, wsID *uuid.UUID, opts port.ListOptions) ([]*domain.TemplateType, string, error)
	updateTypeFn            func(ctx context.Context, tt *domain.TemplateType) error
	getTemplateByIDFn       func(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	getTypeByIDFn           func(ctx context.Context, id uuid.UUID) (*domain.TemplateType, error)
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
func (m *mockTemplateStore) ListByType(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ port.ListOptions) ([]*domain.Template, string, error) {
	return nil, "", nil
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
func (m *mockTemplateStore) DeleteDraftVersion(_ context.Context, _ uuid.UUID) error { return nil }
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
func (m *mockTemplateStore) DeleteLocale(ctx context.Context, versionID uuid.UUID, locale string) error {
	if m.deleteLocaleFn != nil {
		return m.deleteLocaleFn(ctx, versionID, locale)
	}
	return nil
}
func (m *mockTemplateStore) ListTypes(ctx context.Context, wsID *uuid.UUID, opts port.ListOptions) ([]*domain.TemplateType, string, error) {
	if m.listTypesFn != nil {
		return m.listTypesFn(ctx, wsID, opts)
	}
	return nil, "", nil
}
func (m *mockTemplateStore) UpdateType(ctx context.Context, tt *domain.TemplateType) error {
	if m.updateTypeFn != nil {
		return m.updateTypeFn(ctx, tt)
	}
	return nil
}
func (m *mockTemplateStore) SoftDeleteType(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockTemplateStore) GetTemplateByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	if m.getTemplateByIDFn != nil {
		return m.getTemplateByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTemplateStore) GetTypeByID(ctx context.Context, id uuid.UUID) (*domain.TemplateType, error) {
	if m.getTypeByIDFn != nil {
		return m.getTypeByIDFn(ctx, id)
	}
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

// --- Helpers ---

func setupTemplateTypeTest(store port.TemplateStore, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.TemplateTypeHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	svc := service.NewTemplateTypeService(store)
	h := handler.NewTemplateTypeHandler(svc, ts, ws, nil)

	// Workspace-scoped routes.
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/template-types", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/template-types", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/template-types/:slug", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/template-types/:slug", h.Update)

	// Global routes.
	e.POST("/api/v1/manage/global/template-types", h.CreateGlobal)
	e.GET("/api/v1/manage/global/template-types", h.ListGlobal)
	e.GET("/api/v1/manage/global/template-types/:slug", h.GetGlobal)
	e.PUT("/api/v1/manage/global/template-types/:slug", h.UpdateGlobal)

	return e, h
}

// --- Tests ---

func TestTemplateTypeHandler_Create_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var created *domain.TemplateType
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, wsID *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, tt *domain.TemplateType) error {
			created = tt
			return nil
		},
	}

	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"welcome-email","name":"Welcome Email","description":"Welcome onboarding email"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected template type to be created")
	}
	if created.Slug != "welcome-email" {
		t.Fatalf("expected slug 'welcome-email', got %q", created.Slug)
	}
	if created.WorkspaceID == nil || *created.WorkspaceID != ws.ID {
		t.Fatalf("expected workspace ID %s", ws.ID)
	}

	var resp response.TemplateTypeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Slug != "welcome-email" {
		t.Fatalf("expected slug 'welcome-email' in response, got %q", resp.Slug)
	}
}

func TestTemplateTypeHandler_Create_SharedSESRequiresSenderIdentity(t *testing.T) {
	tenant, ws, ts, wsStore := testTenantAndWorkspace()
	systemWSID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		switch id {
		case systemWSID:
			return &domain.Workspace{
				ID:       systemWSID,
				TenantID: tenant.ID,
				Code:     "_system",
				Name:     "System",
				IsSystem: true,
			}, nil
		case ws.ID:
			return ws, nil
		default:
			return nil, domain.ErrNotFound
		}
	}

	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, _ *domain.TemplateType) error {
			t.Fatal("template type should not be persisted when sender identity is missing")
			return nil
		},
	}

	e, h := setupTemplateTypeTest(store, ts, wsStore)

	accessSvc := service.NewAdapterAccessService(
		&mockAdapterStore{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
				if id != adapterID {
					return nil, domain.ErrNotFound
				}
				return &domain.Adapter{
					ID:          adapterID,
					WorkspaceID: &systemWSID,
					Name:        "SES Shared",
					AdapterType: domain.AdapterTypeSES,
				}, nil
			},
		},
		&mockAdapterIdentityStore{},
		wsStore,
		&mockAdapterGrantStoreHandler{},
		&mockIdentityGrantStoreHandler{},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	body := `{"slug":"welcome-email","name":"Welcome Email","adapter_id":"` + adapterID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "sender_identity_id") {
		t.Fatalf("expected sender_identity_id validation error, got %s", rec.Body.String())
	}
}

func TestTemplateTypeHandler_CreateGlobal_Success(t *testing.T) {
	var created *domain.TemplateType
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, wsID *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, tt *domain.TemplateType) error {
			created = tt
			return nil
		},
	}

	e, _ := setupTemplateTypeTest(store, &mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"slug":"password-reset","name":"Password Reset"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/global/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected template type to be created")
	}
	if created.WorkspaceID != nil {
		t.Fatal("expected nil workspace ID for global template type")
	}
}

func TestTemplateTypeHandler_Create_InvalidSlug(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"AB","name":"Invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_Create_MissingName(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"valid-slug"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_Create_DuplicateSlug(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return &domain.TemplateType{}, nil
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"welcome-email","name":"Welcome"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_Create_WithAdapterID(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var created *domain.TemplateType
	adapterID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, tt *domain.TemplateType) error {
			created = tt
			return nil
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"invoice-email","name":"Invoice","adapter_id":"` + adapterID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created.AdapterID == nil || *created.AdapterID != adapterID {
		t.Fatalf("expected adapter ID %s", adapterID)
	}
}

func TestTemplateTypeHandler_Create_InvalidAdapterID(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"invoice-email","name":"Invoice","adapter_id":"not-a-uuid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_Get_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	ttID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error) {
			if slug == "welcome-email" {
				return &domain.TemplateType{
					ID: ttID, WorkspaceID: wsID, Slug: "welcome-email", Name: "Welcome",
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/template-types/welcome-email", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateTypeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Slug != "welcome-email" {
		t.Fatalf("expected slug 'welcome-email', got %q", resp.Slug)
	}
	_ = ws // used indirectly via wsStore
}

func TestTemplateTypeHandler_Get_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/template-types/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_GetGlobal_Success(t *testing.T) {
	ttID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error) {
			if slug == "password-reset" && wsID == nil {
				return &domain.TemplateType{
					ID: ttID, Slug: "password-reset", Name: "Password Reset",
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	e, _ := setupTemplateTypeTest(store, &mockTenantStore{}, &mockWorkspaceStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/template-types/password-reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_Update_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	ttID := uuid.Must(uuid.NewV7())
	original := &domain.TemplateType{
		ID:               ttID,
		WorkspaceID:      &ws.ID,
		Slug:             "welcome-email",
		Name:             "Welcome Email",
		SenderIdentityID: nil,
	}

	var updated *domain.TemplateType
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error) {
			switch slug {
			case "welcome-email":
				return original, nil
			case "welcome-email-v2":
				return nil, domain.ErrNotFound
			default:
				return nil, domain.ErrNotFound
			}
		},
		updateTypeFn: func(_ context.Context, tt *domain.TemplateType) error {
			updated = tt
			return nil
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"name":"Welcome Email v2","slug":"welcome-email-v2"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/template-types/welcome-email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated == nil {
		t.Fatal("expected updated template type")
	}
	if updated.Slug != "welcome-email-v2" {
		t.Fatalf("expected updated slug, got %q", updated.Slug)
	}
	if updated.Name != "Welcome Email v2" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
}

func TestTemplateTypeHandler_Update_InvalidSlug(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	ttID := uuid.Must(uuid.NewV7())
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, slug string, _ *uuid.UUID) (*domain.TemplateType, error) {
			if slug == "welcome-email" {
				return &domain.TemplateType{ID: ttID, WorkspaceID: &ws.ID, Slug: slug, Name: "Welcome"}, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"AB"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/template-types/welcome-email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_List_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{
		listTypesFn: func(_ context.Context, _ *uuid.UUID, _ port.ListOptions) ([]*domain.TemplateType, string, error) {
			return []*domain.TemplateType{}, "", nil
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/template-types", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateTypeListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.HasMore {
		t.Fatal("expected has_more=false for empty list")
	}
}

func TestTemplateTypeHandler_ListGlobal_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()
	now := time.Now().UTC()

	store := &mockTemplateStore{
		listTypesFn: func(_ context.Context, wsID *uuid.UUID, _ port.ListOptions) ([]*domain.TemplateType, string, error) {
			if wsID != nil {
				t.Fatal("expected nil wsID for global scope")
			}
			return []*domain.TemplateType{
				{ID: uuid.New(), Slug: "welcome", Name: "Welcome", CreatedAt: now, UpdatedAt: now},
			}, "next-cursor", nil
		},
	}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/global/template-types", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.TemplateTypeListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
}

func TestTemplateTypeHandler_Create_InvalidBody(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/template-types", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateTypeHandler_Create_InvalidTenant(t *testing.T) {
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}
	wsStore := &mockWorkspaceStore{}
	store := &mockTemplateStore{}
	e, _ := setupTemplateTypeTest(store, ts, wsStore)

	body := `{"slug":"welcome-email","name":"Welcome"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/nonexistent/workspaces/default/template-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

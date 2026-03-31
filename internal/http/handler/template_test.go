package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"context"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// --- Helpers ---

func setupTemplateTest(store port.TemplateStore, compiler port.TemplateCompiler, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.TemplateHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	svc := service.NewTemplateService(store, compiler)
	h := handler.NewTemplateHandler(svc, store, ts, ws, nil)

	base := "/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code"

	e.POST(base+"/templates", h.CreateTemplate)
	e.POST("/api/v1/manage/global/templates", h.CreateTemplateGlobal)
	e.GET(base+"/templates/:template_id/versions", h.ListVersions)
	e.POST(base+"/templates/:template_id/versions", h.CreateVersion)
	e.POST(base+"/templates/:template_id/versions/:version_id/publish", h.PublishVersion)
	e.POST(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.SetLocale)
	e.GET(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.GetLocale)
	e.PUT(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.UpdateLocale)
	e.DELETE(base+"/templates/:template_id/versions/:version_id/locales/:locale", h.DeleteLocale)
	e.POST(base+"/templates/:template_id/preview-mjml", h.PreviewMJML)
	e.POST(base+"/templates/:template_id/disable", h.DisableTemplate)
	e.POST(base+"/templates/:template_id/enable", h.EnableTemplate)
	e.POST("/api/v1/manage/global/templates/:template_id/disable", h.DisableTemplateGlobal)
	e.POST("/api/v1/manage/global/templates/:template_id/enable", h.EnableTemplateGlobal)

	return e, h
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	var created *domain.TemplateVersion
	store := &mockTemplateStore{
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
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

func TestTemplateHandler_PublishVersion_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	versionID := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())
	var publishedID uuid.UUID
	store := &mockTemplateStore{
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/bad-id/publish", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_PublishVersion_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{
		publishFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrNotFound
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	versionID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/publish", nil)
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	templateID := uuid.Must(uuid.NewV7())
	var gotDisabled bool
	store := &mockTemplateStore{
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	var set *domain.TemplateVersionLocale
	store := &mockTemplateStore{
		setLocaleFn: func(_ context.Context, loc *domain.TemplateVersionLocale) error {
			set = loc
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{
		setLocaleFn: func(_ context.Context, _ *domain.TemplateVersionLocale) error {
			return nil
		},
	}

	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	versionID := uuid.Must(uuid.NewV7())
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/"+versionID.String()+"/locales/es", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateHandler_DeleteLocale_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	store := &mockTemplateStore{
		deleteLocaleFn: func(_ context.Context, _ uuid.UUID, _ string) error {
			return domain.ErrNotFound
		},
	}
	e, _ := setupTemplateTest(store, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
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
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupTemplateTest(&mockTemplateStore{}, &mockTemplateCompiler{}, ts, wsStore)

	templateID := uuid.Must(uuid.NewV7())
	body := `{"subject":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/templates/"+templateID.String()+"/versions/not-valid/locales/en", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

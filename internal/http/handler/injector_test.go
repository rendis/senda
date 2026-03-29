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
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

// --- Mock InjectorStore ---

type mockInjectorStore struct {
	createDefinitionFn     func(ctx context.Context, def *domain.InjectorDefinition) error
	getDefinitionByIDFn    func(ctx context.Context, id uuid.UUID) (*domain.InjectorDefinition, error)
	findDefinitionByNameFn func(ctx context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error)
	listDefinitionsInChainFn func(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)
	createFieldFn          func(ctx context.Context, field *domain.InjectorField) error
	getFieldsByDefinitionFn func(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)
	setValueFn             func(ctx context.Context, val *domain.InjectorValue) error
	getValuesFn            func(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)
}

func (m *mockInjectorStore) CreateDefinition(ctx context.Context, def *domain.InjectorDefinition) error {
	if m.createDefinitionFn != nil {
		return m.createDefinitionFn(ctx, def)
	}
	return nil
}
func (m *mockInjectorStore) GetDefinitionByID(ctx context.Context, id uuid.UUID) (*domain.InjectorDefinition, error) {
	if m.getDefinitionByIDFn != nil {
		return m.getDefinitionByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockInjectorStore) FindDefinitionByName(ctx context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error) {
	if m.findDefinitionByNameFn != nil {
		return m.findDefinitionByNameFn(ctx, name, workspaceID)
	}
	return nil, nil
}
func (m *mockInjectorStore) ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
	if m.listDefinitionsInChainFn != nil {
		return m.listDefinitionsInChainFn(ctx, chain)
	}
	return nil, nil
}
func (m *mockInjectorStore) CreateField(ctx context.Context, field *domain.InjectorField) error {
	if m.createFieldFn != nil {
		return m.createFieldFn(ctx, field)
	}
	return nil
}
func (m *mockInjectorStore) GetFieldsByDefinition(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
	if m.getFieldsByDefinitionFn != nil {
		return m.getFieldsByDefinitionFn(ctx, defID)
	}
	return nil, nil
}
func (m *mockInjectorStore) SetValue(ctx context.Context, val *domain.InjectorValue) error {
	if m.setValueFn != nil {
		return m.setValueFn(ctx, val)
	}
	return nil
}
func (m *mockInjectorStore) GetValues(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error) {
	if m.getValuesFn != nil {
		return m.getValuesFn(ctx, defID, chain)
	}
	return nil, nil
}
func (m *mockInjectorStore) GetAllFieldsByDefinitions(_ context.Context, _ []uuid.UUID) (map[uuid.UUID][]*domain.InjectorField, error) {
	return nil, nil
}
func (m *mockInjectorStore) GetAllValuesByDefinitions(_ context.Context, _ []uuid.UUID, _ []uuid.NullUUID) (map[uuid.UUID][]*domain.InjectorValue, error) {
	return nil, nil
}

// --- Helpers ---

func setupInjectorTest(is port.InjectorStore, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.InjectorHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewInjectorHandler(is, ts, ws)

	// Workspace-scoped routes.
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/injectors", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/injectors", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/injectors/:name", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/injectors/:name/values", h.SetValues)

	// Global routes.
	e.POST("/api/v1/manage/global/injectors", h.CreateGlobal)
	e.GET("/api/v1/manage/global/injectors", h.ListGlobal)
	e.GET("/api/v1/manage/global/injectors/:name", h.GetGlobal)

	return e, h
}

func testTenantAndWorkspace() (*domain.Tenant, *domain.Workspace, *mockTenantStore, *mockWorkspaceStore) {
	tenantID := uuid.Must(uuid.NewV7())
	wsID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	tenant := &domain.Tenant{ID: tenantID, Code: "acme", Name: "Acme", CreatedAt: now, UpdatedAt: now}
	ws := &domain.Workspace{ID: wsID, TenantID: tenantID, Code: "default", Name: "Default", CreatedAt: now, UpdatedAt: now}

	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, code string) (*domain.Tenant, error) {
			if code == tenant.Code {
				return tenant, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	wsStore := &mockWorkspaceStore{
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string) (*domain.Workspace, error) {
			if tid == tenantID && code == ws.Code {
				return ws, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	return tenant, ws, ts, wsStore
}

// --- Tests ---

func TestInjectorHandler_Create_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var createdDef *domain.InjectorDefinition
	var createdFields []*domain.InjectorField

	is := &mockInjectorStore{
		createDefinitionFn: func(_ context.Context, def *domain.InjectorDefinition) error {
			createdDef = def
			return nil
		},
		createFieldFn: func(_ context.Context, field *domain.InjectorField) error {
			createdFields = append(createdFields, field)
			return nil
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	body := `{"name":"company_info","description":"Company information","fields":[{"field_name":"company_name","field_type":"text","position":0},{"field_name":"logo_url","field_type":"url","position":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/injectors", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if createdDef == nil {
		t.Fatal("expected definition to be created")
	}
	if createdDef.Name != "company_info" {
		t.Fatalf("expected name 'company_info', got %q", createdDef.Name)
	}
	if len(createdFields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(createdFields))
	}
}

func TestInjectorHandler_List_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	defs := []*domain.InjectorDefinition{
		{ID: uuid.Must(uuid.NewV7()), WorkspaceID: &ws.ID, Name: "def1", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.Must(uuid.NewV7()), WorkspaceID: &ws.ID, Name: "def2", CreatedAt: now, UpdatedAt: now},
	}

	is := &mockInjectorStore{
		listDefinitionsInChainFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return defs, nil
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/injectors", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.InjectorListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
}

func TestInjectorHandler_Get_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	defID := uuid.Must(uuid.NewV7())
	def := &domain.InjectorDefinition{ID: defID, WorkspaceID: &ws.ID, Name: "company_info", CreatedAt: now, UpdatedAt: now}
	fields := []*domain.InjectorField{
		{ID: uuid.Must(uuid.NewV7()), InjectorDefinitionID: defID, FieldName: "company_name", FieldType: domain.FieldTypeText, Position: 0},
	}
	values := []*domain.InjectorValue{
		{ID: uuid.Must(uuid.NewV7()), InjectorDefinitionID: defID, FieldName: "company_name", WorkspaceID: &ws.ID, Value: "Acme Corp", UpdatedAt: now},
	}

	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
			if name == "company_info" {
				return def, nil
			}
			return nil, domain.ErrNotFound
		},
		getFieldsByDefinitionFn: func(_ context.Context, id uuid.UUID) ([]*domain.InjectorField, error) {
			return fields, nil
		},
		getValuesFn: func(_ context.Context, id uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return values, nil
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/injectors/company_info", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.InjectorDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Name != "company_info" {
		t.Fatalf("expected name 'company_info', got %q", resp.Name)
	}
	if len(resp.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(resp.Fields))
	}
	if len(resp.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(resp.Values))
	}
}

func TestInjectorHandler_Get_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/injectors/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInjectorHandler_SetValues_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	defID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	def := &domain.InjectorDefinition{ID: defID, WorkspaceID: &ws.ID, Name: "company_info", CreatedAt: now, UpdatedAt: now}

	var setValues []*domain.InjectorValue
	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
			if name == "company_info" {
				return def, nil
			}
			return nil, domain.ErrNotFound
		},
		setValueFn: func(_ context.Context, val *domain.InjectorValue) error {
			setValues = append(setValues, val)
			return nil
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	body := `{"values":[{"field_name":"company_name","value":"Acme Corp"},{"field_name":"logo_url","value":"https://acme.com/logo.png"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/injectors/company_info/values", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(setValues) != 2 {
		t.Fatalf("expected 2 values set, got %d", len(setValues))
	}
}

func TestInjectorHandler_GlobalCreate_Success(t *testing.T) {
	is := &mockInjectorStore{
		createDefinitionFn: func(_ context.Context, def *domain.InjectorDefinition) error {
			if def.WorkspaceID != nil {
				t.Fatal("expected nil workspace ID for global injector")
			}
			return nil
		},
		createFieldFn: func(_ context.Context, _ *domain.InjectorField) error {
			return nil
		},
	}

	e, _ := setupInjectorTest(is, &mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"name":"global_branding","fields":[{"field_name":"brand_color","field_type":"text","position":0}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/global/injectors", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

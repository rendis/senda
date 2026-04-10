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
)

// --- Mock InjectorStore ---

type mockInjectorStore struct {
	createDefinitionFn       func(ctx context.Context, def *domain.InjectorDefinition) error
	updateDefinitionSchemaFn func(ctx context.Context, currentName string, workspaceID *uuid.UUID, def *domain.InjectorDefinition, fields []*domain.InjectorField) error
	getDefinitionByIDFn      func(ctx context.Context, id uuid.UUID) (*domain.InjectorDefinition, error)
	findDefinitionByNameFn   func(ctx context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error)
	listDefinitionsInChainFn func(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)
	createFieldFn            func(ctx context.Context, field *domain.InjectorField) error
	updateFieldFn            func(ctx context.Context, field *domain.InjectorField) error
	getFieldsByDefinitionFn  func(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)
	setValueFn               func(ctx context.Context, val *domain.InjectorValue) error
	getValuesFn              func(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)
}

func (m *mockInjectorStore) CreateDefinition(ctx context.Context, def *domain.InjectorDefinition) error {
	if m.createDefinitionFn != nil {
		return m.createDefinitionFn(ctx, def)
	}
	return nil
}
func (m *mockInjectorStore) UpdateDefinitionSchema(ctx context.Context, currentName string, workspaceID *uuid.UUID, def *domain.InjectorDefinition, fields []*domain.InjectorField) error {
	if m.updateDefinitionSchemaFn != nil {
		return m.updateDefinitionSchemaFn(ctx, currentName, workspaceID, def, fields)
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
func (m *mockInjectorStore) SoftDeleteDefinition(_ context.Context, _ uuid.UUID) error { return nil }
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
func (m *mockInjectorStore) UpdateField(ctx context.Context, field *domain.InjectorField) error {
	if m.updateFieldFn != nil {
		return m.updateFieldFn(ctx, field)
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
	return map[uuid.UUID][]*domain.InjectorField{}, nil
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
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/injectors/:name", h.Update)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/injectors/:name/values", h.SetValues)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/injectors/:name/fields/:field_name", h.UpdateField)

	// Global routes.
	e.POST("/api/v1/manage/global/injectors", h.CreateGlobal)
	e.GET("/api/v1/manage/global/injectors", h.ListGlobal)
	e.GET("/api/v1/manage/global/injectors/:name", h.GetGlobal)
	e.PUT("/api/v1/manage/global/injectors/:name", h.UpdateGlobal)
	e.PUT("/api/v1/manage/global/injectors/:name/fields/:field_name", h.UpdateFieldGlobal)

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
		getByTenantAndCodeFn: func(_ context.Context, tid uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
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

	body := `{"name":"company_info","description":"Company information","fields":[{"field_name":"company_name","field_type":"text","position":0,"default_value":"Acme Corp","allow_overwrite":false},{"field_name":"logo_url","field_type":"url","position":1,"allow_overwrite":true}]}`
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
	if createdFields[0].DefaultValue != "Acme Corp" {
		t.Fatalf("expected first field default value, got %#v", createdFields[0].DefaultValue)
	}
	if createdFields[0].AllowOverwrite {
		t.Fatal("expected first field to disable overwrite")
	}
	if !createdFields[1].AllowOverwrite {
		t.Fatal("expected second field to allow overwrite")
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

func TestInjectorHandler_List_IncludeInheritedUsesResolutionChain(t *testing.T) {
	tenant, ws, ts, wsStore := testTenantAndWorkspace()
	systemID := uuid.Must(uuid.NewV7())

wsStore.getSystemWorkspaceFn = func(_ context.Context, tenantID uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			t.Fatalf("unexpected tenant id %s", tenantID)
		}
		return &domain.Workspace{
			ID:       systemID,
			TenantID: tenant.ID,
			Code:     "_system",
			Name:     "System",
			IsSystem: true,
		}, nil
	}

	var gotChain []uuid.NullUUID
	is := &mockInjectorStore{
		listDefinitionsInChainFn: func(_ context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			gotChain = append([]uuid.NullUUID(nil), chain...)
			return []*domain.InjectorDefinition{}, nil
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/manage/tenants/acme/workspaces/default/injectors?include_inherited=true",
		nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(gotChain) != 2 {
		t.Fatalf("expected workspace/system chain, got %+v", gotChain)
	}
	if !gotChain[0].Valid || gotChain[0].UUID != ws.ID {
		t.Fatalf("expected workspace scope first, got %+v", gotChain)
	}
	if !gotChain[1].Valid || gotChain[1].UUID != systemID {
		t.Fatalf("expected system scope second, got %+v", gotChain)
	}
}

func TestInjectorHandler_Get_ResolvesInheritedSystemInjectorInWorkspace(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemID := uuid.Must(uuid.NewV7())
	defID := uuid.Must(uuid.NewV7())

wsStore.getSystemWorkspaceFn = func(_ context.Context, tenantID uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			t.Fatalf("unexpected tenant id %s", tenantID)
		}
		return &domain.Workspace{
			ID:       systemID,
			TenantID: tenant.ID,
			Code:     "_system",
			Name:     "Default",
			IsSystem: true,
		}, nil
	}

	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error) {
			if workspaceID == nil || *workspaceID != systemID {
				return nil, domain.ErrNotFound
			}
			if name != "company_info" {
				t.Fatalf("unexpected name %q", name)
			}
			return &domain.InjectorDefinition{
				ID:          defID,
				WorkspaceID: &systemID,
				Name:        "company_info",
			}, nil
		},
		getFieldsByDefinitionFn: func(_ context.Context, id uuid.UUID) ([]*domain.InjectorField, error) {
			if id != defID {
				t.Fatalf("expected definition id %s, got %s", defID, id)
			}
			return []*domain.InjectorField{
				{ID: uuid.Must(uuid.NewV7()), InjectorDefinitionID: defID, FieldName: "company_name", FieldType: domain.FieldTypeText, Position: 0},
			}, nil
		},
		getValuesFn: func(_ context.Context, id uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			if id != defID {
				t.Fatalf("expected definition id %s, got %s", defID, id)
			}
			return nil, nil
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
		t.Fatalf("decode error: %v", err)
	}
	if resp.OwnerScope != "system" {
		t.Fatalf("expected owner_scope system, got %q", resp.OwnerScope)
	}
	if !resp.InheritedFromSystem {
		t.Fatal("expected inherited system marker")
	}
}

func TestInjectorHandler_Get_DoesNotFallbackToGlobalInjectorInWorkspace(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemID := uuid.Must(uuid.NewV7())

wsStore.getSystemWorkspaceFn = func(_ context.Context, tenantID uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			t.Fatalf("unexpected tenant id %s", tenantID)
		}
		return &domain.Workspace{
			ID:       systemID,
			TenantID: tenant.ID,
			Code:     "_system",
			Name:     "Default",
			IsSystem: true,
		}, nil
	}

	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error) {
			if name != "global_only" {
				t.Fatalf("unexpected name %q", name)
			}
			if workspaceID == nil {
				return &domain.InjectorDefinition{
					ID:   uuid.Must(uuid.NewV7()),
					Name: "global_only",
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/injectors/global_only", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInjectorHandler_Get_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	defID := uuid.Must(uuid.NewV7())
	def := &domain.InjectorDefinition{ID: defID, WorkspaceID: &ws.ID, Name: "company_info", CreatedAt: now, UpdatedAt: now}
	fields := []*domain.InjectorField{
		{
			ID:                   uuid.Must(uuid.NewV7()),
			InjectorDefinitionID: defID,
			FieldName:            "company_name",
			FieldType:            domain.FieldTypeText,
			Position:             0,
			DefaultValue:         "Acme Corp",
			AllowOverwrite:       true,
		},
	}
	values := []*domain.InjectorValue{
		{
			ID:                   uuid.Must(uuid.NewV7()),
			InjectorDefinitionID: defID,
			FieldName:            "company_name",
			WorkspaceID:          &ws.ID,
			Value:                `"Workspace Value"`,
			UpdatedAt:            now,
		},
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
		getValuesFn: func(_ context.Context, id uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			if id != defID {
				t.Fatalf("unexpected definition lookup %s", id)
			}
			if len(chain) != 1 || !chain[0].Valid || chain[0].UUID != ws.ID {
				t.Fatalf("unexpected chain %+v", chain)
			}
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
	if resp.Fields[0].DefaultValue != "Acme Corp" {
		t.Fatalf("expected field default value in response, got %#v", resp.Fields[0].DefaultValue)
	}
	if !resp.Fields[0].AllowOverwrite {
		t.Fatal("expected allow_overwrite in response")
	}
	if len(resp.Values) != 1 {
		t.Fatalf("expected 1 scoped value, got %d", len(resp.Values))
	}
	if resp.Values[0].Value != `"Workspace Value"` {
		t.Fatalf("expected scoped value in response, got %q", resp.Values[0].Value)
	}
}

func TestInjectorHandler_Create_RejectsMissingDefaultWhenOverwriteDisabled(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	is := &mockInjectorStore{}
	e, _ := setupInjectorTest(is, ts, wsStore)

	body := `{"name":"student","fields":[{"field_name":"name","field_type":"text","position":0,"allow_overwrite":false}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/injectors", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
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

func TestInjectorHandler_UpdateField_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	defID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	def := &domain.InjectorDefinition{ID: defID, WorkspaceID: &ws.ID, Name: "student", CreatedAt: now, UpdatedAt: now}
	fields := []*domain.InjectorField{
		{
			ID:                   uuid.Must(uuid.NewV7()),
			InjectorDefinitionID: defID,
			FieldName:            "name",
			FieldType:            domain.FieldTypeText,
			Position:             0,
			DefaultValue:         "Old Name",
			AllowOverwrite:       true,
		},
	}

	var updated *domain.InjectorField
	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
			if name == "student" {
				return def, nil
			}
			return nil, domain.ErrNotFound
		},
		getFieldsByDefinitionFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return fields, nil
		},
		updateFieldFn: func(_ context.Context, field *domain.InjectorField) error {
			updated = field
			return nil
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	body := `{"default_value":"Updated Name","allow_overwrite":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/injectors/student/fields/name", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated == nil {
		t.Fatal("expected updated field")
	}
	if updated.DefaultValue != "Updated Name" {
		t.Fatalf("expected updated default, got %#v", updated.DefaultValue)
	}
	if updated.AllowOverwrite {
		t.Fatal("expected overwrite to be disabled")
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

func TestInjectorHandler_GlobalUpdateField_Success(t *testing.T) {
	now := time.Now().UTC()
	defID := uuid.Must(uuid.NewV7())
	def := &domain.InjectorDefinition{ID: defID, Name: "global_branding", CreatedAt: now, UpdatedAt: now}
	fields := []*domain.InjectorField{
		{
			ID:                   uuid.Must(uuid.NewV7()),
			InjectorDefinitionID: defID,
			FieldName:            "brand_color",
			FieldType:            domain.FieldTypeText,
			Position:             0,
			DefaultValue:         "blue",
			AllowOverwrite:       true,
		},
	}

	var updated *domain.InjectorField
	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error) {
			if workspaceID != nil {
				t.Fatalf("expected global lookup, got workspace scope %v", *workspaceID)
			}
			if name != "global_branding" {
				return nil, domain.ErrNotFound
			}
			return def, nil
		},
		getFieldsByDefinitionFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return fields, nil
		},
		updateFieldFn: func(_ context.Context, field *domain.InjectorField) error {
			updated = field
			return nil
		},
	}

	e, _ := setupInjectorTest(is, &mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"default_value":"teal","allow_overwrite":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/global/injectors/global_branding/fields/brand_color", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated == nil {
		t.Fatal("expected global field to be updated")
	}
	if updated.DefaultValue != "teal" {
		t.Fatalf("expected updated default, got %#v", updated.DefaultValue)
	}
	if updated.AllowOverwrite {
		t.Fatal("expected overwrite to be disabled")
	}
}

func TestInjectorHandler_Update_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var (
		currentName    string
		gotWorkspaceID *uuid.UUID
		updatedDef     *domain.InjectorDefinition
		updatedFields  []*domain.InjectorField
	)

	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error) {
			if name != "student" {
				t.Fatalf("expected lookup for student, got %q", name)
			}
			if workspaceID == nil || *workspaceID != ws.ID {
				t.Fatalf("expected workspace lookup %s, got %v", ws.ID, workspaceID)
			}
			return &domain.InjectorDefinition{
				ID:          uuid.Must(uuid.NewV7()),
				WorkspaceID: &ws.ID,
				Name:        name,
			}, nil
		},
		updateDefinitionSchemaFn: func(_ context.Context, name string, workspaceID *uuid.UUID, def *domain.InjectorDefinition, fields []*domain.InjectorField) error {
			currentName = name
			gotWorkspaceID = workspaceID
			updatedDef = def
			updatedFields = fields
			def.ID = uuid.Must(uuid.NewV7())
			def.CreatedAt = time.Now().UTC()
			def.UpdatedAt = time.Now().UTC()
			for _, field := range fields {
				field.InjectorDefinitionID = def.ID
			}
			return nil
		},
	}

	e, _ := setupInjectorTest(is, ts, wsStore)

	body := `{"name":"student_profile","description":"Student profile","fields":[{"field_name":"full name","field_type":"text","description":"Display name","position":0,"default_value":"Ada","allow_overwrite":true},{"field_name":"age","field_type":"number","position":1,"default_value":18,"allow_overwrite":false}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/injectors/student", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if currentName != "student" {
		t.Fatalf("expected current name student, got %q", currentName)
	}
	if gotWorkspaceID == nil || *gotWorkspaceID != ws.ID {
		t.Fatalf("expected workspace scope %v, got %v", ws.ID, gotWorkspaceID)
	}
	if updatedDef == nil || updatedDef.Name != "student_profile" {
		t.Fatalf("expected updated def name, got %#v", updatedDef)
	}
	if updatedDef.Description == nil || *updatedDef.Description != "Student profile" {
		t.Fatalf("expected updated description, got %#v", updatedDef.Description)
	}
	if len(updatedFields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(updatedFields))
	}
	if updatedFields[0].FieldName != "full name" {
		t.Fatalf("expected first field rename, got %q", updatedFields[0].FieldName)
	}
	if updatedFields[1].FieldType != domain.FieldTypeNumber {
		t.Fatalf("expected second field type number, got %q", updatedFields[1].FieldType)
	}
	if updatedFields[1].AllowOverwrite {
		t.Fatal("expected second field overwrite disabled")
	}
}

func TestInjectorHandler_Update_RejectsDuplicateFieldNames(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	is := &mockInjectorStore{
		findDefinitionByNameFn: func(_ context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error) {
			if name != "student" {
				t.Fatalf("expected lookup for student, got %q", name)
			}
			if workspaceID == nil || *workspaceID != ws.ID {
				t.Fatalf("expected workspace lookup %s, got %v", ws.ID, workspaceID)
			}
			return &domain.InjectorDefinition{
				ID:          uuid.Must(uuid.NewV7()),
				WorkspaceID: &ws.ID,
				Name:        name,
			}, nil
		},
	}
	e, _ := setupInjectorTest(is, ts, wsStore)

	body := `{"name":"student","fields":[{"field_name":"full name","field_type":"text","position":0},{"field_name":"full name","field_type":"text","position":1}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/injectors/student", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInjectorHandler_GlobalUpdate_Success(t *testing.T) {
	var (
		currentName    string
		gotWorkspaceID *uuid.UUID
		updatedDef     *domain.InjectorDefinition
		updatedFields  []*domain.InjectorField
	)

	is := &mockInjectorStore{
		updateDefinitionSchemaFn: func(_ context.Context, name string, workspaceID *uuid.UUID, def *domain.InjectorDefinition, fields []*domain.InjectorField) error {
			currentName = name
			gotWorkspaceID = workspaceID
			updatedDef = def
			updatedFields = fields
			def.ID = uuid.Must(uuid.NewV7())
			def.CreatedAt = time.Now().UTC()
			def.UpdatedAt = time.Now().UTC()
			for _, field := range fields {
				field.InjectorDefinitionID = def.ID
			}
			return nil
		},
	}

	e, _ := setupInjectorTest(is, &mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"name":"global_branding_v2","description":"Brand palette","fields":[{"field_name":"brand color","field_type":"text","position":0,"default_value":"teal","allow_overwrite":false}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/global/injectors/global_branding", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if currentName != "global_branding" {
		t.Fatalf("expected current name global_branding, got %q", currentName)
	}
	if gotWorkspaceID != nil {
		t.Fatalf("expected global scope, got %v", gotWorkspaceID)
	}
	if updatedDef == nil || updatedDef.Name != "global_branding_v2" {
		t.Fatalf("expected renamed global injector, got %#v", updatedDef)
	}
	if len(updatedFields) != 1 || updatedFields[0].FieldName != "brand color" {
		t.Fatalf("expected updated global field payload, got %#v", updatedFields)
	}
}

package resolution_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/resolution"
	"github.com/senda-app/senda/pkg/apperr"
)

// --- Mock InjectorStore ---

type mockInjectorStore struct {
	listDefsFn  func(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)
	getFieldsFn func(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)
	getValuesFn func(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)
}

func (m *mockInjectorStore) CreateDefinition(_ context.Context, _ *domain.InjectorDefinition) error {
	return nil
}
func (m *mockInjectorStore) GetDefinitionByID(_ context.Context, _ uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStore) FindDefinitionByName(_ context.Context, _ string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStore) ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
	return m.listDefsFn(ctx, chain)
}
func (m *mockInjectorStore) CreateField(_ context.Context, _ *domain.InjectorField) error { return nil }
func (m *mockInjectorStore) GetFieldsByDefinition(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
	return m.getFieldsFn(ctx, defID)
}
func (m *mockInjectorStore) SetValue(_ context.Context, _ *domain.InjectorValue) error { return nil }
func (m *mockInjectorStore) GetValues(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error) {
	return m.getValuesFn(ctx, defID, chain)
}
func (m *mockInjectorStore) GetAllFieldsByDefinitions(ctx context.Context, defIDs []uuid.UUID) (map[uuid.UUID][]*domain.InjectorField, error) {
	result := make(map[uuid.UUID][]*domain.InjectorField, len(defIDs))
	for _, id := range defIDs {
		fields, err := m.getFieldsFn(ctx, id)
		if err != nil {
			return nil, err
		}
		result[id] = fields
	}
	return result, nil
}
func (m *mockInjectorStore) GetAllValuesByDefinitions(ctx context.Context, defIDs []uuid.UUID, chain []uuid.NullUUID) (map[uuid.UUID][]*domain.InjectorValue, error) {
	result := make(map[uuid.UUID][]*domain.InjectorValue, len(defIDs))
	for _, id := range defIDs {
		values, err := m.getValuesFn(ctx, id, chain)
		if err != nil {
			return nil, err
		}
		result[id] = values
	}
	return result, nil
}

// --- Helper to build a mock ChainResolver that returns a pre-built chain ---

func newTestChainResolver(chain *resolution.ResolutionChain, err error) *resolution.ChainResolver {
	wsID := chain.WorkspaceID
	sysID := chain.SystemWorkspaceID
	tenantID := chain.TenantID

	store := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			if err != nil {
				return nil, err
			}
			return &domain.Workspace{ID: wsID, TenantID: tenantID, IsSystem: false}, nil
		},
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return &domain.Workspace{ID: sysID, TenantID: tenantID, IsSystem: true}, nil
		},
	}
	return resolution.NewChainResolver(store, newMockCache())
}

func newErrorChainResolver(retErr error) *resolution.ChainResolver {
	store := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, retErr
		},
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, nil
		},
	}
	return resolution.NewChainResolver(store, newMockCache())
}

// --- Tests ---

func TestInjectorMerger_SingleScopeGlobal(t *testing.T) {
	defID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo_url", FieldType: domain.FieldTypeURL, Position: 0},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo_url", WorkspaceID: nil, Value: `"https://example.com/logo.png"`},
			}, nil
		},
	}

	tenantID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brand, ok := result["brand"]
	if !ok {
		t.Fatal("expected 'brand' in result")
	}
	logo, ok := brand["logo_url"]
	if !ok {
		t.Fatal("expected 'logo_url' field in brand")
	}
	if logo != "https://example.com/logo.png" {
		t.Errorf("logo_url = %v, want 'https://example.com/logo.png'", logo)
	}
}

func TestInjectorMerger_WorkspaceOverridesGlobal(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "color", FieldType: domain.FieldTypeText, Position: 0},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "color", WorkspaceID: nil, Value: `"blue"`},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "color", WorkspaceID: &wsID, Value: `"red"`},
			}, nil
		},
	}

	tenantID := uuid.New()
	sysID := uuid.New()
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	color := result["brand"]["color"]
	if color != "red" {
		t.Errorf("color = %v, want 'red' (workspace override)", color)
	}
}

func TestInjectorMerger_ThreeLevelMerge(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", FieldType: domain.FieldTypeURL, Position: 0},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "color", FieldType: domain.FieldTypeText, Position: 1},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "footer", FieldType: domain.FieldTypeHTML, Position: 2},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				// Global sets all three
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", WorkspaceID: nil, Value: `"global-logo"`},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "color", WorkspaceID: nil, Value: `"global-color"`},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "footer", WorkspaceID: nil, Value: `"global-footer"`},
				// System overrides color
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "color", WorkspaceID: &sysID, Value: `"system-color"`},
				// Workspace overrides logo
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", WorkspaceID: &wsID, Value: `"ws-logo"`},
			}, nil
		},
	}

	tenantID := uuid.New()
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brand := result["brand"]
	if brand["logo"] != "ws-logo" {
		t.Errorf("logo = %v, want 'ws-logo' (workspace override)", brand["logo"])
	}
	if brand["color"] != "system-color" {
		t.Errorf("color = %v, want 'system-color' (system override)", brand["color"])
	}
	if brand["footer"] != "global-footer" {
		t.Errorf("footer = %v, want 'global-footer' (global fallback)", brand["footer"])
	}
}

func TestInjectorMerger_FieldWithNoValue(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", FieldType: domain.FieldTypeURL, Position: 0},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "unset_field", FieldType: domain.FieldTypeText, Position: 1},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", WorkspaceID: nil, Value: `"logo-val"`},
				// no value for "unset_field" at any scope
			}, nil
		},
	}

	tenantID := uuid.New()
	sysID := uuid.New()
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brand := result["brand"]
	if brand["logo"] != "logo-val" {
		t.Errorf("logo = %v, want 'logo-val'", brand["logo"])
	}
	// unset_field should be present with nil value
	val, exists := brand["unset_field"]
	if !exists {
		t.Error("expected 'unset_field' key to exist in result")
	}
	if val != nil {
		t.Errorf("unset_field = %v, want nil", val)
	}
}

func TestInjectorMerger_MultipleDefinitions(t *testing.T) {
	defA := uuid.New()
	defB := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defA, WorkspaceID: nil, Name: "brand"},
				{ID: defB, WorkspaceID: nil, Name: "social"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
			if defID == defA {
				return []*domain.InjectorField{
					{ID: uuid.New(), InjectorDefinitionID: defA, FieldName: "logo", FieldType: domain.FieldTypeURL},
				}, nil
			}
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defB, FieldName: "twitter", FieldType: domain.FieldTypeURL},
			}, nil
		},
		getValuesFn: func(_ context.Context, defID uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			if defID == defA {
				return []*domain.InjectorValue{
					{ID: uuid.New(), InjectorDefinitionID: defA, FieldName: "logo", WorkspaceID: nil, Value: `"logo.png"`},
				}, nil
			}
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defB, FieldName: "twitter", WorkspaceID: nil, Value: `"@senda"`},
			}, nil
		},
	}

	wsID := uuid.New()
	sysID := uuid.New()
	tenantID := uuid.New()
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("result has %d injectors, want 2", len(result))
	}
	if result["brand"]["logo"] != "logo.png" {
		t.Errorf("brand.logo = %v, want 'logo.png'", result["brand"]["logo"])
	}
	if result["social"]["twitter"] != "@senda" {
		t.Errorf("social.twitter = %v, want '@senda'", result["social"]["twitter"])
	}
}

func TestInjectorMerger_DuplicateDefNames_WorkspaceWins(t *testing.T) {
	globalDefID := uuid.New()
	wsDefID := uuid.New()
	wsID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: globalDefID, WorkspaceID: nil, Name: "brand"},
				{ID: wsDefID, WorkspaceID: &wsID, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
			// Only the workspace def should be queried
			if defID == wsDefID {
				return []*domain.InjectorField{
					{ID: uuid.New(), InjectorDefinitionID: wsDefID, FieldName: "logo", FieldType: domain.FieldTypeURL},
				}, nil
			}
			t.Errorf("unexpected GetFieldsByDefinition call for defID %v (should only call for ws def)", defID)
			return nil, nil
		},
		getValuesFn: func(_ context.Context, defID uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			if defID == wsDefID {
				return []*domain.InjectorValue{
					{ID: uuid.New(), InjectorDefinitionID: wsDefID, FieldName: "logo", WorkspaceID: &wsID, Value: `"ws-logo"`},
				}, nil
			}
			return nil, nil
		},
	}

	tenantID := uuid.New()
	sysID := uuid.New()
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["brand"]["logo"] != "ws-logo" {
		t.Errorf("brand.logo = %v, want 'ws-logo' (workspace def should win)", result["brand"]["logo"])
	}
}

func TestInjectorMerger_ChainResolverError(t *testing.T) {
	injStore := &mockInjectorStore{
		listDefsFn:  func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) { return nil, nil },
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) { return nil, nil },
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
	}

	cr := newErrorChainResolver(apperr.NotFound("workspace not found"))
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	_, err := merger.Resolve(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestInjectorMerger_StoreError(t *testing.T) {
	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return nil, apperr.Internal("db error")
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) { return nil, nil },
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
	}

	wsID := uuid.New()
	sysID := uuid.New()
	tenantID := uuid.New()
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	_, err := merger.Resolve(context.Background(), wsID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 500 {
		t.Errorf("expected Internal error, got %v", err)
	}
}

// --- Code Injector tests ---

type stubCodeInjector struct {
	code       string
	resolveFn  port.CodeResolveFunc
	fields     map[string]any
	deps       []string
	critical   bool
	err        error
}

func (s *stubCodeInjector) Code() string { return s.code }
func (s *stubCodeInjector) Resolve() (port.CodeResolveFunc, []string) {
	if s.resolveFn != nil {
		return s.resolveFn, s.deps
	}
	return func(_ context.Context, _ *port.InjectorContext) (map[string]any, error) {
		if s.err != nil {
			return nil, s.err
		}
		return s.fields, nil
	}, s.deps
}
func (s *stubCodeInjector) IsCritical() bool       { return s.critical }
func (s *stubCodeInjector) Timeout() time.Duration { return 0 }

func emptyInjStore() *mockInjectorStore {
	return &mockInjectorStore{
		listDefsFn:  func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) { return nil, nil },
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) { return nil, nil },
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) { return nil, nil },
	}
}

func TestResolveWithContext_CodeInjectorsOnly(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	codeInj := &stubCodeInjector{
		code:   "student",
		fields: map[string]any{"name": "Jane", "email": "jane@test.com"},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{codeInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:welcome", nil, uuid.Nil, wsID, "welcome")

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["student"]["name"] != "Jane" {
		t.Errorf("student.name = %v, want Jane", result["student"]["name"])
	}
	if result["student"]["email"] != "jane@test.com" {
		t.Errorf("student.email = %v, want jane@test.com", result["student"]["email"])
	}
}

func TestResolveWithContext_InitFunc(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	initFunc := func(_ context.Context, _ *port.InjectorContext) (any, error) {
		return "loaded-data", nil
	}

	inj := &stubCodeInjector{
		code: "from_init",
		resolveFn: func(_ context.Context, injCtx *port.InjectorContext) (map[string]any, error) {
			data := injCtx.InitData().(string)
			return map[string]any{"value": data}, nil
		},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{inj}, initFunc)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["from_init"]["value"] != "loaded-data" {
		t.Errorf("from_init.value = %v, want loaded-data", result["from_init"]["value"])
	}
}

func TestResolveWithContext_NonCriticalSkipped(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	failInj := &stubCodeInjector{code: "flaky", critical: false, err: errors.New("network timeout")}
	okInj := &stubCodeInjector{code: "stable", fields: map[string]any{"ok": true}}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{failInj, okInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("non-critical failure should not abort: %v", err)
	}
	if _, exists := result["flaky"]; exists {
		t.Error("flaky injector should not appear in results")
	}
	if result["stable"]["ok"] != true {
		t.Error("stable injector should still resolve")
	}
}

func TestResolveWithContext_CriticalAborts(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	criticalInj := &stubCodeInjector{code: "must_work", critical: true, err: errors.New("db down")}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{criticalInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	_, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err == nil {
		t.Fatal("critical injector failure should abort")
	}
}

func TestResolveWithContext_DependencyOrder(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	var order []string
	parentInj := &orderTracker{code: "parent", fields: map[string]any{"v": "1"}, order: &order}
	childInj := &orderTracker{code: "child", deps: []string{"parent"}, fields: map[string]any{"v": "2"}, order: &order}

	cr := newTestChainResolver(chain, nil)
	// Register child BEFORE parent to test dependency resolution.
	merger := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{childInj, parentInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	_, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != "parent" || order[1] != "child" {
		t.Errorf("execution order = %v, want [parent, child]", order)
	}
}

type orderTracker struct {
	code   string
	deps   []string
	fields map[string]any
	order  *[]string
}

func (o *orderTracker) Code() string { return o.code }
func (o *orderTracker) Resolve() (port.CodeResolveFunc, []string) {
	return func(_ context.Context, _ *port.InjectorContext) (map[string]any, error) {
		*o.order = append(*o.order, o.code)
		return o.fields, nil
	}, o.deps
}
func (o *orderTracker) IsCritical() bool       { return false }
func (o *orderTracker) Timeout() time.Duration { return 0 }

func TestResolveWithContext_CodeOverridesDB(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}, {Valid: false}},
	}

	// DB injector named "brand" with logo field.
	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", FieldType: domain.FieldTypeURL, Position: 0},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", WorkspaceID: nil, Value: `"db-logo.png"`},
			}, nil
		},
	}

	// Code injector with same name "brand" → should override DB.
	codeInj := &stubCodeInjector{
		code:   "brand",
		fields: map[string]any{"logo": "code-logo.png", "extra": "from-code"},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, []port.CodeInjector{codeInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Code injector should override DB.
	if result["brand"]["logo"] != "code-logo.png" {
		t.Errorf("brand.logo = %v, want code-logo.png", result["brand"]["logo"])
	}
	if result["brand"]["extra"] != "from-code" {
		t.Errorf("brand.extra = %v, want from-code", result["brand"]["extra"])
	}
}

func TestResolveWithContext_MixedDBAndCode(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}, {Valid: false}},
	}

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "company"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", FieldType: domain.FieldTypeText, Position: 0},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", WorkspaceID: nil, Value: `"Acme"`},
			}, nil
		},
	}

	codeInj := &stubCodeInjector{
		code:   "student",
		fields: map[string]any{"full_name": "Jane Doe"},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, []port.CodeInjector{codeInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both DB and code injectors should be present.
	if result["company"]["name"] != "Acme" {
		t.Errorf("company.name = %v, want Acme", result["company"]["name"])
	}
	if result["student"]["full_name"] != "Jane Doe" {
		t.Errorf("student.full_name = %v, want Jane Doe", result["student"]["full_name"])
	}
}

func TestResolveWithContext_NoCodeInjectors_SameAsResolve(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}, {Valid: false}},
	}

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", FieldType: domain.FieldTypeText, Position: 0},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", WorkspaceID: nil, Value: `"Acme"`},
			}, nil
		},
	}

	cr := newTestChainResolver(chain, nil)
	// No code injectors.
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	resultCtx, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("ResolveWithContext error: %v", err)
	}
	resultPlain, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	if resultCtx["brand"]["name"] != resultPlain["brand"]["name"] {
		t.Errorf("ResolveWithContext and Resolve should return same DB values")
	}
}

func TestHasCodeInjectors(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}
	cr := newTestChainResolver(chain, nil)

	// Without code injectors.
	m1 := resolution.NewInjectorMerger(emptyInjStore(), cr, nil, nil)
	if m1.HasCodeInjectors() {
		t.Error("HasCodeInjectors should be false when no injectors/initFunc")
	}

	// With code injectors.
	m2 := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{
		&stubCodeInjector{code: "x", fields: map[string]any{}},
	}, nil)
	if !m2.HasCodeInjectors() {
		t.Error("HasCodeInjectors should be true when injectors registered")
	}

	// With only initFunc.
	m3 := resolution.NewInjectorMerger(emptyInjStore(), cr, nil, func(_ context.Context, _ *port.InjectorContext) (any, error) {
		return nil, nil
	})
	if !m3.HasCodeInjectors() {
		t.Error("HasCodeInjectors should be true when initFunc is set")
	}
}

func TestResolveWithContext_InitFuncError(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	failInit := func(_ context.Context, _ *port.InjectorContext) (any, error) {
		return nil, errors.New("init failed: db unreachable")
	}

	inj := &stubCodeInjector{code: "x", fields: map[string]any{"v": 1}}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{inj}, failInit)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	_, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err == nil {
		t.Fatal("InitFunc error should abort resolution")
	}
	if !strings.Contains(err.Error(), "init failed") {
		t.Errorf("error should contain init message, got: %v", err)
	}
}

func TestResolveWithContext_DepOnDBInjector(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}, {Valid: false}},
	}

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: nil, Name: "company"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", FieldType: domain.FieldTypeText, Position: 0},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", WorkspaceID: nil, Value: `"Acme"`},
			}, nil
		},
	}

	// Code injector that declares dep on "company" (a DB injector).
	// The dep is not in code injectors, so it's skipped during dep resolution.
	// But the DB value should be accessible via GetResolved since it was seeded.
	codeInj := &stubCodeInjector{
		code: "derived",
		deps: []string{"company"}, // DB injector
		resolveFn: func(_ context.Context, injCtx *port.InjectorContext) (map[string]any, error) {
			company, _ := injCtx.GetResolved("company")
			name := ""
			if company != nil {
				if n, ok := company["name"].(string); ok {
					name = n
				}
			}
			return map[string]any{"greeting": "Hello from " + name}, nil
		},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, []port.CodeInjector{codeInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["derived"]["greeting"] != "Hello from Acme" {
		t.Errorf("derived.greeting = %v, want 'Hello from Acme'", result["derived"]["greeting"])
	}
}

func TestResolveWithContext_DuplicateCodeInjectorCodes(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	// Two code injectors with same Code. Last one registered wins
	// because the byCode map overwrites.
	first := &stubCodeInjector{code: "dup", fields: map[string]any{"v": "first"}}
	second := &stubCodeInjector{code: "dup", fields: map[string]any{"v": "second"}}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(emptyInjStore(), cr, []port.CodeInjector{first, second}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:t", nil, uuid.Nil, wsID, "t")

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Last registered wins (map overwrite).
	if result["dup"]["v"] != "second" {
		t.Errorf("dup.v = %v, want 'second' (last registered wins)", result["dup"]["v"])
	}
}

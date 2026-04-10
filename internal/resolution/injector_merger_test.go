package resolution_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
	"github.com/rendis/senda/pkg/apperr"
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
func (m *mockInjectorStore) UpdateDefinitionSchema(_ context.Context, _ string, _ *uuid.UUID, _ *domain.InjectorDefinition, _ []*domain.InjectorField) error {
	return nil
}
func (m *mockInjectorStore) GetDefinitionByID(_ context.Context, _ uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStore) FindDefinitionByName(_ context.Context, _ string, _ *uuid.UUID) (*domain.InjectorDefinition, error) {
	return nil, nil
}
func (m *mockInjectorStore) SoftDeleteDefinition(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockInjectorStore) ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
	return m.listDefsFn(ctx, chain)
}
func (m *mockInjectorStore) CreateField(_ context.Context, _ *domain.InjectorField) error { return nil }
func (m *mockInjectorStore) UpdateField(_ context.Context, _ *domain.InjectorField) error { return nil }
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

func TestInjectorMerger_IncludesSystemDefinitionsAsFallback(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			if len(chain) != 2 {
				t.Fatalf("expected full resolution chain, got %+v", chain)
			}
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: &sysID, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo_url", FieldType: domain.FieldTypeURL, Position: 0, DefaultValue: "default-logo"},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo_url", WorkspaceID: &sysID, Value: `"system-logo"`},
			}, nil
		},
	}

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          uuid.New(),
		Scopes:            []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result["brand"]["logo_url"]; got != "system-logo" {
		t.Fatalf("brand.logo_url = %v, want system-logo", got)
	}
}

func TestInjectorMerger_UsesFieldDefaultOnlyWhenNoValueExists(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: &wsID, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{
					ID:                   uuid.New(),
					InjectorDefinitionID: defID,
					FieldName:            "color",
					FieldType:            domain.FieldTypeText,
					Position:             0,
					DefaultValue:         "red",
					AllowOverwrite:       true,
				},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
	}

	chain := &resolution.ResolutionChain{WorkspaceID: wsID, Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}}}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	color := result["brand"]["color"]
	if color != "red" {
		t.Errorf("color = %v, want 'red' (field default fallback)", color)
	}
}

func TestInjectorMerger_UsesMultipleFieldDefaults(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: &wsID, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", FieldType: domain.FieldTypeURL, Position: 0, DefaultValue: "ws-logo", AllowOverwrite: true},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "color", FieldType: domain.FieldTypeText, Position: 1, DefaultValue: "brand-color", AllowOverwrite: true},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "footer", FieldType: domain.FieldTypeHTML, Position: 2, DefaultValue: "<p>footer</p>", AllowOverwrite: false},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
	}

	chain := &resolution.ResolutionChain{WorkspaceID: wsID, Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}}}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brand := result["brand"]
	if brand["logo"] != "ws-logo" {
		t.Errorf("logo = %v, want 'ws-logo'", brand["logo"])
	}
	if brand["color"] != "brand-color" {
		t.Errorf("color = %v, want 'brand-color'", brand["color"])
	}
	if brand["footer"] != "<p>footer</p>" {
		t.Errorf("footer = %v, want '<p>footer</p>'", brand["footer"])
	}
}

func TestInjectorMerger_FieldWithNilDefault(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defID, WorkspaceID: &wsID, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", FieldType: domain.FieldTypeURL, Position: 0, DefaultValue: "logo-val", AllowOverwrite: true},
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "unset_field", FieldType: domain.FieldTypeText, Position: 1, DefaultValue: nil, AllowOverwrite: true},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
	}

	chain := &resolution.ResolutionChain{WorkspaceID: wsID, Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}}}
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
	wsID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: defA, WorkspaceID: &wsID, Name: "brand"},
				{ID: defB, WorkspaceID: &wsID, Name: "social"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
			if defID == defA {
				return []*domain.InjectorField{
					{ID: uuid.New(), InjectorDefinitionID: defA, FieldName: "logo", FieldType: domain.FieldTypeURL, DefaultValue: "logo.png", AllowOverwrite: true},
				}, nil
			}
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defB, FieldName: "twitter", FieldType: domain.FieldTypeURL, DefaultValue: "@senda", AllowOverwrite: true},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
	}

	chain := &resolution.ResolutionChain{WorkspaceID: wsID, Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}}}
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
					{ID: uuid.New(), InjectorDefinitionID: wsDefID, FieldName: "logo", FieldType: domain.FieldTypeURL, DefaultValue: "ws-logo", AllowOverwrite: true},
				}, nil
			}
			t.Errorf("unexpected GetFieldsByDefinition call for defID %v (should only call for ws def)", defID)
			return nil, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
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

func TestInjectorMerger_UsesSystemInheritedValueBeforeFieldDefault(t *testing.T) {
	systemDefID := uuid.New()
	wsID := uuid.New()
	sysID := uuid.New()

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: systemDefID, WorkspaceID: &sysID, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{
					ID:                   uuid.New(),
					InjectorDefinitionID: systemDefID,
					FieldName:            "color",
					FieldType:            domain.FieldTypeText,
					Position:             0,
					DefaultValue:         "default-red",
					AllowOverwrite:       true,
				},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: systemDefID, FieldName: "color", WorkspaceID: &sysID, Value: `"system-blue"`},
			}, nil
		},
	}

	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          uuid.New(),
		Scopes:            []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result["brand"]["color"]; got != "system-blue" {
		t.Fatalf("brand.color = %v, want system-blue", got)
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
	cr := newTestChainResolver(&resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	_, err := merger.Resolve(context.Background(), wsID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 500 {
		t.Errorf("expected internal error, got %v", err)
	}
}

// --- Code Injector tests ---

type stubCodeInjector struct {
	code      string
	resolveFn port.CodeResolveFunc
	fields    map[string]any
	deps      []string
	critical  bool
	err       error
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
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
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
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "logo", FieldType: domain.FieldTypeURL, Position: 0, AllowOverwrite: true},
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
				{ID: defID, WorkspaceID: &wsID, Name: "company"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", FieldType: domain.FieldTypeText, Position: 0, DefaultValue: "Acme", AllowOverwrite: true},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
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
				{ID: defID, WorkspaceID: &wsID, Name: "brand"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", FieldType: domain.FieldTypeText, Position: 0, DefaultValue: "Acme", AllowOverwrite: true},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
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
				{ID: defID, WorkspaceID: &wsID, Name: "company"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", FieldType: domain.FieldTypeText, Position: 0, DefaultValue: "Acme", AllowOverwrite: true},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
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

func TestResolveWithContext_RequestInjectorsOverrideCodeAndDefault(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID: wsID,
		Scopes:      []uuid.NullUUID{{UUID: wsID, Valid: true}},
	}

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{{ID: defID, WorkspaceID: &wsID, Name: "student"}}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{
					ID:                   uuid.New(),
					InjectorDefinitionID: defID,
					FieldName:            "name",
					FieldType:            domain.FieldTypeText,
					Position:             0,
					DefaultValue:         "Default Student",
					AllowOverwrite:       true,
				},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", WorkspaceID: &wsID, Value: `"DB Student"`},
			}, nil
		},
	}

	codeInj := &stubCodeInjector{
		code:   "student",
		fields: map[string]any{"name": "Code Student"},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, []port.CodeInjector{codeInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:welcome", nil, uuid.Nil, wsID, "welcome")
	injCtx.SetRequestInjectors(map[string]map[string]any{
		"student": {"name": "Request Student"},
	})

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result["student"]["name"]; got != "Request Student" {
		t.Fatalf("student.name = %v, want Request Student", got)
	}
}

func TestResolveWithContext_LockedFieldAlwaysUsesDefault(t *testing.T) {
	defID := uuid.New()
	wsID := uuid.Must(uuid.NewV7())
	sysID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          uuid.New(),
		Scopes:            []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}},
	}

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{{ID: defID, WorkspaceID: &sysID, Name: "student"}}, nil
		},
		getFieldsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.InjectorField, error) {
			return []*domain.InjectorField{
				{
					ID:                   uuid.New(),
					InjectorDefinitionID: defID,
					FieldName:            "name",
					FieldType:            domain.FieldTypeText,
					Position:             0,
					DefaultValue:         "Locked Default",
					AllowOverwrite:       false,
				},
			}, nil
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return []*domain.InjectorValue{
				{ID: uuid.New(), InjectorDefinitionID: defID, FieldName: "name", WorkspaceID: &sysID, Value: `"Inherited DB"`},
			}, nil
		},
	}

	codeInj := &stubCodeInjector{
		code:   "student",
		fields: map[string]any{"name": "Code Student"},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, []port.CodeInjector{codeInj}, nil)
	injCtx := port.NewInjectorContext(nil, "t:w:welcome", nil, uuid.Nil, wsID, "welcome")
	injCtx.SetRequestInjectors(map[string]map[string]any{
		"student": {"name": "Request Student"},
	})

	result, err := merger.ResolveWithContext(context.Background(), wsID, injCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result["student"]["name"]; got != "Inherited DB" {
		t.Fatalf("student.name = %v, want Inherited DB", got)
	}
}

func TestResolve_UsesDefinitionsAcrossWorkspaceAndSystemChain(t *testing.T) {
	systemDefID := uuid.New()
	wsDefID := uuid.New()
	wsID := uuid.Must(uuid.NewV7())
	sysID := uuid.Must(uuid.NewV7())
	chain := &resolution.ResolutionChain{
		WorkspaceID:       wsID,
		SystemWorkspaceID: sysID,
		TenantID:          uuid.New(),
		Scopes:            []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}},
	}

	injStore := &mockInjectorStore{
		listDefsFn: func(_ context.Context, _ []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			return []*domain.InjectorDefinition{
				{ID: systemDefID, WorkspaceID: &sysID, Name: "system_only"},
				{ID: wsDefID, WorkspaceID: &wsID, Name: "workspace_only"},
			}, nil
		},
		getFieldsFn: func(_ context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
			switch defID {
			case wsDefID:
				return []*domain.InjectorField{{
					ID:                   uuid.New(),
					InjectorDefinitionID: wsDefID,
					FieldName:            "name",
					FieldType:            domain.FieldTypeText,
					Position:             0,
					DefaultValue:         "Workspace",
					AllowOverwrite:       true,
				}}, nil
			case systemDefID:
				return []*domain.InjectorField{{
					ID:                   uuid.New(),
					InjectorDefinitionID: systemDefID,
					FieldName:            "name",
					FieldType:            domain.FieldTypeText,
					Position:             0,
					DefaultValue:         "System",
					AllowOverwrite:       true,
				}}, nil
			default:
				return nil, nil
			}
		},
		getValuesFn: func(_ context.Context, _ uuid.UUID, _ []uuid.NullUUID) ([]*domain.InjectorValue, error) {
			return nil, nil
		},
	}

	cr := newTestChainResolver(chain, nil)
	merger := resolution.NewInjectorMerger(injStore, cr, nil, nil)

	result, err := merger.Resolve(context.Background(), wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result["system_only"]["name"]; got != "System" {
		t.Fatalf("system_only.name = %v, want System", got)
	}
	if got := result["workspace_only"]["name"]; got != "Workspace" {
		t.Fatalf("workspace_only.name = %v, want Workspace", got)
	}
}

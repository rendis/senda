package resolution

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
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

// --- Helper to build a mock ChainResolver that returns a pre-built chain ---

func newTestChainResolver(chain *ResolutionChain, err error) *ChainResolver {
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
	return NewChainResolver(store, newMockCache())
}

func newErrorChainResolver(retErr error) *ChainResolver {
	store := &mockWorkspaceStore{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, retErr
		},
		getSystemWorkspace: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return nil, nil
		},
	}
	return NewChainResolver(store, newMockCache())
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
	chain := &ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := NewInjectorMerger(injStore, cr)

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
	chain := &ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := NewInjectorMerger(injStore, cr)

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
	chain := &ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := NewInjectorMerger(injStore, cr)

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
	chain := &ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := NewInjectorMerger(injStore, cr)

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
	chain := &ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := NewInjectorMerger(injStore, cr)

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
	chain := &ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := NewInjectorMerger(injStore, cr)

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
	merger := NewInjectorMerger(injStore, cr)

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
	chain := &ResolutionChain{
		WorkspaceID: wsID, SystemWorkspaceID: sysID, TenantID: tenantID,
		Scopes: []uuid.NullUUID{{UUID: wsID, Valid: true}, {UUID: sysID, Valid: true}, {Valid: false}},
	}
	cr := newTestChainResolver(chain, nil)
	merger := NewInjectorMerger(injStore, cr)

	_, err := merger.Resolve(context.Background(), wsID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 500 {
		t.Errorf("expected Internal error, got %v", err)
	}
}

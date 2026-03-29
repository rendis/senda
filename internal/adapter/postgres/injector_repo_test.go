//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

// createTestWorkspaceWith creates a tenant + workspace for FK chains in tests.
func createTestWorkspaceWith(ctx context.Context, t *testing.T, tenantRepo *pgadapter.TenantRepo, wsRepo *pgadapter.WorkspaceRepo) *domain.Workspace {
	t.Helper()
	tenant := createTestTenant(ctx, t, tenantRepo)
	ws := &domain.Workspace{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Code:     "ws-" + uuid.New().String()[:8],
		Name:     "Test Workspace",
	}
	if err := wsRepo.Create(ctx, ws); err != nil {
		t.Fatalf("creating test workspace: %v", err)
	}
	return ws
}

// --- InjectorDefinition tests ---

func TestInjectorRepo_CreateDefinition(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	def := &domain.InjectorDefinition{
		ID:          uuid.New(),
		WorkspaceID: &ws.ID,
		Name:        "header",
	}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}
	if def.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if def.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestInjectorRepo_CreateDefinition_Global(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	def := &domain.InjectorDefinition{
		ID:   uuid.New(),
		Name: "global-inj",
	}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}
	if def.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestInjectorRepo_CreateDefinition_Conflict(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	def1 := &domain.InjectorDefinition{
		ID:   uuid.New(),
		Name: "dup-def",
	}
	if err := repo.CreateDefinition(ctx, def1); err != nil {
		t.Fatalf("first CreateDefinition() error: %v", err)
	}

	def2 := &domain.InjectorDefinition{
		ID:   uuid.New(),
		Name: "dup-def",
	}
	err := repo.CreateDefinition(ctx, def2)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestInjectorRepo_GetDefinitionByID(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	desc := "A test injector"
	def := &domain.InjectorDefinition{
		ID:          uuid.New(),
		Name:        "get-def",
		Description: &desc,
	}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	got, err := repo.GetDefinitionByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("GetDefinitionByID() error: %v", err)
	}
	if got.Name != "get-def" {
		t.Errorf("want name get-def, got %q", got.Name)
	}
	if got.Description == nil || *got.Description != desc {
		t.Errorf("want description %q, got %v", desc, got.Description)
	}
}

func TestInjectorRepo_GetDefinitionByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	_, err := repo.GetDefinitionByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestInjectorRepo_FindDefinitionByName(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	// Create global definition
	globalDef := &domain.InjectorDefinition{
		ID:   uuid.New(),
		Name: "shared-def",
	}
	if err := repo.CreateDefinition(ctx, globalDef); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	// Create workspace definition with same name
	wsDef := &domain.InjectorDefinition{
		ID:          uuid.New(),
		WorkspaceID: &ws.ID,
		Name:        "shared-def",
	}
	if err := repo.CreateDefinition(ctx, wsDef); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	// Find global
	got, err := repo.FindDefinitionByName(ctx, "shared-def", nil)
	if err != nil {
		t.Fatalf("FindDefinitionByName(global) error: %v", err)
	}
	if got.ID != globalDef.ID {
		t.Errorf("expected global def ID %s, got %s", globalDef.ID, got.ID)
	}

	// Find workspace-scoped
	got, err = repo.FindDefinitionByName(ctx, "shared-def", &ws.ID)
	if err != nil {
		t.Fatalf("FindDefinitionByName(ws) error: %v", err)
	}
	if got.ID != wsDef.ID {
		t.Errorf("expected ws def ID %s, got %s", wsDef.ID, got.ID)
	}
}

func TestInjectorRepo_FindDefinitionByName_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	_, err := repo.FindDefinitionByName(ctx, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestInjectorRepo_ListDefinitionsInChain(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	// Global def
	globalDef := &domain.InjectorDefinition{ID: uuid.New(), Name: "chain-g"}
	if err := repo.CreateDefinition(ctx, globalDef); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	// Workspace def
	wsDef := &domain.InjectorDefinition{ID: uuid.New(), WorkspaceID: &ws.ID, Name: "chain-ws"}
	if err := repo.CreateDefinition(ctx, wsDef); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	chain := []uuid.NullUUID{
		{UUID: ws.ID, Valid: true},
		{Valid: false}, // global
	}

	defs, err := repo.ListDefinitionsInChain(ctx, chain)
	if err != nil {
		t.Fatalf("ListDefinitionsInChain() error: %v", err)
	}
	if len(defs) < 2 {
		t.Fatalf("expected at least 2 definitions, got %d", len(defs))
	}

	ids := make(map[uuid.UUID]bool)
	for _, d := range defs {
		ids[d.ID] = true
	}
	if !ids[globalDef.ID] {
		t.Error("expected global definition in chain results")
	}
	if !ids[wsDef.ID] {
		t.Error("expected workspace definition in chain results")
	}
}

// --- InjectorField tests ---

func TestInjectorRepo_CreateField(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	def := &domain.InjectorDefinition{ID: uuid.New(), Name: "field-def-" + uuid.New().String()[:8]}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	field := &domain.InjectorField{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "company_name",
		FieldType:            domain.FieldTypeText,
		Position:             0,
	}
	if err := repo.CreateField(ctx, field); err != nil {
		t.Fatalf("CreateField() error: %v", err)
	}
}

func TestInjectorRepo_CreateField_Conflict(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	def := &domain.InjectorDefinition{ID: uuid.New(), Name: "field-dup-" + uuid.New().String()[:8]}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	field := &domain.InjectorField{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "dup_field",
		FieldType:            domain.FieldTypeText,
	}
	if err := repo.CreateField(ctx, field); err != nil {
		t.Fatalf("first CreateField() error: %v", err)
	}

	field2 := &domain.InjectorField{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "dup_field",
		FieldType:            domain.FieldTypeText,
	}
	err := repo.CreateField(ctx, field2)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestInjectorRepo_GetFieldsByDefinition(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	def := &domain.InjectorDefinition{ID: uuid.New(), Name: "fields-def-" + uuid.New().String()[:8]}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	desc := "The logo URL"
	for i, name := range []string{"logo", "company_name", "footer"} {
		f := &domain.InjectorField{
			ID:                   uuid.New(),
			InjectorDefinitionID: def.ID,
			FieldName:            name,
			FieldType:            domain.FieldTypeText,
			Position:             i,
		}
		if name == "logo" {
			f.FieldType = domain.FieldTypeImg
			f.Description = &desc
		}
		if err := repo.CreateField(ctx, f); err != nil {
			t.Fatalf("CreateField(%s) error: %v", name, err)
		}
	}

	fields, err := repo.GetFieldsByDefinition(ctx, def.ID)
	if err != nil {
		t.Fatalf("GetFieldsByDefinition() error: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	// Verify ordered by position
	if fields[0].FieldName != "logo" {
		t.Errorf("expected first field to be 'logo', got %q", fields[0].FieldName)
	}
	if fields[0].FieldType != domain.FieldTypeImg {
		t.Errorf("expected first field type to be img, got %q", fields[0].FieldType)
	}
	if fields[0].Description == nil || *fields[0].Description != desc {
		t.Errorf("expected description %q, got %v", desc, fields[0].Description)
	}
	if fields[1].FieldName != "company_name" {
		t.Errorf("expected second field to be 'company_name', got %q", fields[1].FieldName)
	}
	if fields[2].FieldName != "footer" {
		t.Errorf("expected third field to be 'footer', got %q", fields[2].FieldName)
	}
}

// --- InjectorValue tests ---

func TestInjectorRepo_SetValue(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	def := &domain.InjectorDefinition{ID: uuid.New(), Name: "val-def-" + uuid.New().String()[:8]}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	field := &domain.InjectorField{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "company",
		FieldType:            domain.FieldTypeText,
	}
	if err := repo.CreateField(ctx, field); err != nil {
		t.Fatalf("CreateField() error: %v", err)
	}

	val := &domain.InjectorValue{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "company",
		Value:                "Acme Inc",
	}
	if err := repo.SetValue(ctx, val); err != nil {
		t.Fatalf("SetValue() error: %v", err)
	}
	if val.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestInjectorRepo_SetValue_Upsert(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)

	def := &domain.InjectorDefinition{ID: uuid.New(), Name: "upsert-def-" + uuid.New().String()[:8]}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	field := &domain.InjectorField{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "brand",
		FieldType:            domain.FieldTypeText,
	}
	if err := repo.CreateField(ctx, field); err != nil {
		t.Fatalf("CreateField() error: %v", err)
	}

	val := &domain.InjectorValue{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "brand",
		Value:                "v1",
	}
	if err := repo.SetValue(ctx, val); err != nil {
		t.Fatalf("SetValue() error: %v", err)
	}
	originalID := val.ID

	// Upsert with new value
	val2 := &domain.InjectorValue{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "brand",
		Value:                "v2",
	}
	if err := repo.SetValue(ctx, val2); err != nil {
		t.Fatalf("SetValue(upsert) error: %v", err)
	}
	// Should keep original ID from ON CONFLICT
	if val2.ID != originalID {
		t.Errorf("expected upserted ID %s, got %s", originalID, val2.ID)
	}

	// Verify updated value
	chain := []uuid.NullUUID{{Valid: false}}
	vals, err := repo.GetValues(ctx, def.ID, chain)
	if err != nil {
		t.Fatalf("GetValues() error: %v", err)
	}
	if len(vals) != 1 {
		t.Fatalf("expected 1 value, got %d", len(vals))
	}
	if vals[0].Value != "v2" {
		t.Errorf("expected value v2, got %q", vals[0].Value)
	}
}

func TestInjectorRepo_GetValues(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewInjectorRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	def := &domain.InjectorDefinition{ID: uuid.New(), Name: "getval-def-" + uuid.New().String()[:8]}
	if err := repo.CreateDefinition(ctx, def); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}

	field := &domain.InjectorField{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "color",
		FieldType:            domain.FieldTypeText,
	}
	if err := repo.CreateField(ctx, field); err != nil {
		t.Fatalf("CreateField() error: %v", err)
	}

	// Global value
	globalVal := &domain.InjectorValue{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "color",
		Value:                "blue",
	}
	if err := repo.SetValue(ctx, globalVal); err != nil {
		t.Fatalf("SetValue(global) error: %v", err)
	}

	// Workspace-scoped value
	wsVal := &domain.InjectorValue{
		ID:                   uuid.New(),
		InjectorDefinitionID: def.ID,
		FieldName:            "color",
		WorkspaceID:          &ws.ID,
		Value:                "red",
	}
	if err := repo.SetValue(ctx, wsVal); err != nil {
		t.Fatalf("SetValue(ws) error: %v", err)
	}

	// Get values in chain
	chain := []uuid.NullUUID{
		{UUID: ws.ID, Valid: true},
		{Valid: false}, // global
	}

	vals, err := repo.GetValues(ctx, def.ID, chain)
	if err != nil {
		t.Fatalf("GetValues() error: %v", err)
	}
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d", len(vals))
	}

	found := make(map[string]bool)
	for _, v := range vals {
		found[v.Value] = true
	}
	if !found["blue"] {
		t.Error("expected global value 'blue' in results")
	}
	if !found["red"] {
		t.Error("expected workspace value 'red' in results")
	}
}

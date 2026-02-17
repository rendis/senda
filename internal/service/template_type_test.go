package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/service"
)

func TestTemplateTypeService_Create_Success(t *testing.T) {
	var created *domain.TemplateType
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, tt *domain.TemplateType) error {
			created = tt
			return nil
		},
	}

	svc := service.NewTemplateTypeService(store)

	wsID := uuid.Must(uuid.NewV7())
	desc := "Welcome emails"
	tt, err := svc.Create(context.Background(), "welcome-email", "Welcome Email", &desc, nil, nil, &wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt.Slug != "welcome-email" {
		t.Fatalf("expected slug 'welcome-email', got %q", tt.Slug)
	}
	if tt.Name != "Welcome Email" {
		t.Fatalf("expected name 'Welcome Email', got %q", tt.Name)
	}
	if tt.WorkspaceID == nil || *tt.WorkspaceID != wsID {
		t.Fatalf("expected workspace ID %s", wsID)
	}
	if created == nil {
		t.Fatal("expected template type to be persisted")
	}
}

func TestTemplateTypeService_Create_Global(t *testing.T) {
	var created *domain.TemplateType
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, tt *domain.TemplateType) error {
			created = tt
			return nil
		},
	}

	svc := service.NewTemplateTypeService(store)

	tt, err := svc.Create(context.Background(), "password-reset", "Password Reset", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt.WorkspaceID != nil {
		t.Fatal("expected nil workspace ID for global template type")
	}
	if created == nil {
		t.Fatal("expected template type to be persisted")
	}
}

func TestTemplateTypeService_Create_DuplicateSlug(t *testing.T) {
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return &domain.TemplateType{}, nil
		},
	}

	svc := service.NewTemplateTypeService(store)

	_, err := svc.Create(context.Background(), "welcome-email", "Welcome", nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for duplicate slug")
	}
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestTemplateTypeService_Create_StoreError(t *testing.T) {
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, _ *domain.TemplateType) error {
			return errors.New("db error")
		},
	}

	svc := service.NewTemplateTypeService(store)

	_, err := svc.Create(context.Background(), "test-type", "Test", nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "db error" {
		t.Fatalf("expected 'db error', got %q", err.Error())
	}
}

func TestTemplateTypeService_Create_WithAdapterAndSchema(t *testing.T) {
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
		createTypeFn: func(_ context.Context, _ *domain.TemplateType) error {
			return nil
		},
	}

	svc := service.NewTemplateTypeService(store)

	adapterID := uuid.Must(uuid.NewV7())
	schema := map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}

	tt, err := svc.Create(context.Background(), "invoice", "Invoice", nil, &adapterID, schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt.AdapterID == nil || *tt.AdapterID != adapterID {
		t.Fatalf("expected adapter ID %s", adapterID)
	}
	if tt.VariableSchema == nil {
		t.Fatal("expected variable schema to be set")
	}
}

func TestTemplateTypeService_GetBySlug_Success(t *testing.T) {
	expected := &domain.TemplateType{ID: uuid.Must(uuid.NewV7()), Slug: "welcome-email"}
	store := &mockTemplateStore{
		getTypeBySlugFn: func(_ context.Context, slug string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			if slug == "welcome-email" {
				return expected, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	svc := service.NewTemplateTypeService(store)

	tt, err := svc.GetBySlug(context.Background(), "welcome-email", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt.ID != expected.ID {
		t.Fatalf("expected ID %s, got %s", expected.ID, tt.ID)
	}
}

func TestTemplateTypeService_GetBySlug_NotFound(t *testing.T) {
	store := &mockTemplateStore{
		getTypeBySlugFn: func(_ context.Context, _ string, _ []uuid.NullUUID) (*domain.TemplateType, error) {
			return nil, domain.ErrNotFound
		},
	}

	svc := service.NewTemplateTypeService(store)

	_, err := svc.GetBySlug(context.Background(), "nonexistent", nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTemplateTypeService_FindBySlugInScope_Success(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	expected := &domain.TemplateType{ID: uuid.Must(uuid.NewV7()), Slug: "invoice", WorkspaceID: &wsID}
	store := &mockTemplateStore{
		findTypeBySlugInScopeFn: func(_ context.Context, slug string, ws *uuid.UUID) (*domain.TemplateType, error) {
			if slug == "invoice" && ws != nil && *ws == wsID {
				return expected, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	svc := service.NewTemplateTypeService(store)

	tt, err := svc.FindBySlugInScope(context.Background(), "invoice", &wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt.ID != expected.ID {
		t.Fatalf("expected ID %s, got %s", expected.ID, tt.ID)
	}
}

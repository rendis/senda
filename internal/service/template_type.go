package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/pkg/apperr"
)

// TemplateTypeService handles template type business logic.
// It only requires TemplateTypeStore (the narrowest sub-interface of TemplateStore).
type TemplateTypeService struct {
	store port.TemplateTypeStore
}

// NewTemplateTypeService creates a new TemplateTypeService.
// Accepts port.TemplateTypeStore — any port.TemplateStore also satisfies this.
func NewTemplateTypeService(store port.TemplateTypeStore) *TemplateTypeService {
	return &TemplateTypeService{store: store}
}

// Create creates a new template type scoped to a workspace (or global if wsID is nil).
func (s *TemplateTypeService) Create(ctx context.Context, slug, name string, description *string, adapterID *uuid.UUID, variableSchema map[string]any, wsID *uuid.UUID) (*domain.TemplateType, error) {
	existing, err := s.store.FindTypeBySlugInScope(ctx, slug, wsID)
	if err != nil && !isNotFoundErr(err) {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: template type with slug %q already exists in scope", domain.ErrConflict, slug)
	}

	now := time.Now().UTC()
	tt := &domain.TemplateType{
		ID:             uuid.Must(uuid.NewV7()),
		WorkspaceID:    wsID,
		Slug:           slug,
		Name:           name,
		Description:    description,
		AdapterID:      adapterID,
		VariableSchema: variableSchema,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.store.CreateType(ctx, tt); err != nil {
		return nil, err
	}

	return tt, nil
}

// GetBySlug retrieves a template type by slug within a resolution chain.
func (s *TemplateTypeService) GetBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error) {
	return s.store.GetTypeBySlug(ctx, slug, chain)
}

// FindBySlugInScope retrieves a template type by slug within a specific scope.
func (s *TemplateTypeService) FindBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error) {
	return s.store.FindTypeBySlugInScope(ctx, slug, wsID)
}

// ListTypes lists template types in a scope (nil wsID = global).
func (s *TemplateTypeService) ListTypes(ctx context.Context, wsID *uuid.UUID, opts port.ListOptions) ([]*domain.TemplateType, string, error) {
	return s.store.ListTypes(ctx, wsID, opts)
}

// isNotFoundErr returns true if err represents a "not found" condition,
// checking both domain sentinel errors and apperr typed errors.
func isNotFoundErr(err error) bool {
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrTemplateTypeNotFound) {
		return true
	}
	var appErr *apperr.AppError
	return errors.As(err, &appErr) && appErr.Code == http.StatusNotFound
}

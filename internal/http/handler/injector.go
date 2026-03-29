package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// InjectorHandler handles CRUD operations for injector definitions and values.
type InjectorHandler struct {
	store   port.InjectorStore
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewInjectorHandler creates a new InjectorHandler.
func NewInjectorHandler(is port.InjectorStore, ts port.TenantStore, ws port.WorkspaceStore) *InjectorHandler {
	return &InjectorHandler{store: is, tsStore: ts, wsStore: ws}
}

// Create handles POST /tenants/:tenant_code/workspaces/:workspace_code/injectors.
func (h *InjectorHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.create(c, &ws.ID)
}

// CreateGlobal handles POST /global/injectors.
func (h *InjectorHandler) CreateGlobal(c *echo.Context) error {
	return h.create(c, nil)
}

func (h *InjectorHandler) create(c *echo.Context, workspaceID *uuid.UUID) error {
	var req request.CreateInjectorRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.Name == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	}
	if len(req.Fields) == 0 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "fields", Message: "at least one field is required"})
	}
	for i, f := range req.Fields {
		if f.FieldName == "" {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "fields", Message: "field_name is required at index " + itoa(i)})
		}
		if !isValidFieldType(f.FieldType) {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "fields", Message: "invalid field_type at index " + itoa(i)})
		}
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	ctx := c.Request().Context()
	now := time.Now().UTC()

	def := &domain.InjectorDefinition{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.CreateDefinition(ctx, def); err != nil {
		return mapStoreError(c, err)
	}

	fields := make([]*domain.InjectorField, len(req.Fields))
	for i, f := range req.Fields {
		field := &domain.InjectorField{
			ID:                   uuid.Must(uuid.NewV7()),
			InjectorDefinitionID: def.ID,
			FieldName:            f.FieldName,
			FieldType:            domain.InjectorFieldType(f.FieldType),
			Description:          f.Description,
			Position:             f.Position,
		}
		if err := h.store.CreateField(ctx, field); err != nil {
			return mapStoreError(c, err)
		}
		fields[i] = field
	}

	return c.JSON(http.StatusCreated, response.NewInjectorCreateResponse(def, fields))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/injectors.
func (h *InjectorHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.list(c, &ws.ID)
}

// ListGlobal handles GET /global/injectors.
func (h *InjectorHandler) ListGlobal(c *echo.Context) error {
	return h.list(c, nil)
}

func (h *InjectorHandler) list(c *echo.Context, workspaceID *uuid.UUID) error {
	ctx := c.Request().Context()

	// Use ListDefinitionsInChain with a single scope to list definitions at this level only.
	chain := []uuid.NullUUID{uuidToNullUUID(workspaceID)}
	defs, err := h.store.ListDefinitionsInChain(ctx, chain)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewInjectorListResponse(defs))
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/injectors/:name.
func (h *InjectorHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.get(c, &ws.ID)
}

// GetGlobal handles GET /global/injectors/:name.
func (h *InjectorHandler) GetGlobal(c *echo.Context) error {
	return h.get(c, nil)
}

func (h *InjectorHandler) get(c *echo.Context, workspaceID *uuid.UUID) error {
	name := c.Param("name")
	ctx := c.Request().Context()

	def, err := h.store.FindDefinitionByName(ctx, name, workspaceID)
	if err != nil {
		return mapStoreError(c, err)
	}

	fields, err := h.store.GetFieldsByDefinition(ctx, def.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	chain := []uuid.NullUUID{uuidToNullUUID(workspaceID)}
	values, err := h.store.GetValues(ctx, def.ID, chain)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewInjectorDetailResponse(def, fields, values))
}

// SetValues handles PUT /tenants/:tenant_code/workspaces/:workspace_code/injectors/:name/values.
func (h *InjectorHandler) SetValues(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.setValues(c, &ws.ID)
}

func (h *InjectorHandler) setValues(c *echo.Context, workspaceID *uuid.UUID) error {
	name := c.Param("name")
	ctx := c.Request().Context()

	def, err := h.store.FindDefinitionByName(ctx, name, workspaceID)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.SetInjectorValuesRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if len(req.Values) == 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "values", Message: "at least one value is required"},
		)
	}

	now := time.Now().UTC()
	for _, fv := range req.Values {
		val := &domain.InjectorValue{
			ID:                   uuid.Must(uuid.NewV7()),
			InjectorDefinitionID: def.ID,
			FieldName:            fv.FieldName,
			WorkspaceID:          workspaceID,
			Value:                fv.Value,
			UpdatedAt:            now,
		}
		if err := h.store.SetValue(ctx, val); err != nil {
			return mapStoreError(c, err)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

func isValidFieldType(ft string) bool {
	switch domain.InjectorFieldType(ft) {
	case domain.FieldTypeText, domain.FieldTypeNumber, domain.FieldTypeBool,
		domain.FieldTypeImg, domain.FieldTypeURL, domain.FieldTypeHTML:
		return true
	}
	return false
}

func uuidToNullUUID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

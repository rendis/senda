package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
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
	if !ws.IsSystem {
		systemWorkspace, err := resolveSystemWorkspace(c.Request().Context(), ws, h.wsStore)
		if err != nil {
			return mapStoreError(c, err)
		}
		if !effectiveWorkspacePolicies(systemWorkspace).AllowWorkspaceLocalInjectors {
			return writePolicyForbidden(c, "WORKSPACE_LOCAL_INJECTORS_DISABLED", "workspace local injectors are disabled by tenant policy")
		}
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

	fieldErrors := validateInjectorSchemaRequest(req)
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
		allowOverwrite := true
		if f.AllowOverwrite != nil {
			allowOverwrite = *f.AllowOverwrite
		}
		field := &domain.InjectorField{
			ID:                   uuid.Must(uuid.NewV7()),
			InjectorDefinitionID: def.ID,
			FieldName:            f.FieldName,
			FieldType:            domain.InjectorFieldType(f.FieldType),
			Description:          f.Description,
			Position:             f.Position,
			DefaultValue:         f.DefaultValue,
			AllowOverwrite:       allowOverwrite,
		}
		if err := h.store.CreateField(ctx, field); err != nil {
			return mapStoreError(c, err)
		}
		fields[i] = field
	}

	return c.JSON(http.StatusCreated, response.NewInjectorCreateResponse(def, fields))
}

// Update handles PUT /tenants/:tenant_code/workspaces/:workspace_code/injectors/:name.
func (h *InjectorHandler) Update(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.update(c, ws)
}

// UpdateGlobal handles PUT /global/injectors/:name.
func (h *InjectorHandler) UpdateGlobal(c *echo.Context) error {
	return h.update(c, nil)
}

func (h *InjectorHandler) update(c *echo.Context, workspace *domain.Workspace) error {
	currentName := c.Param("name")
	var workspaceID *uuid.UUID
	if workspace != nil {
		workspaceID = &workspace.ID
		def, _, policies, err := h.loadWorkspaceDefinitionAccess(c.Request().Context(), workspace, currentName)
		if err != nil {
			return mapStoreError(c, err)
		}
		if !isOwnedByCurrentWorkspace(def.WorkspaceID, workspace) {
			return writePolicyForbidden(c, "READ_ONLY_INHERITED_INJECTOR", "inherited injectors are read-only in workspace scope")
		}
		if !workspace.IsSystem && !policies.AllowWorkspaceLocalInjectors {
			return writePolicyForbidden(c, "WORKSPACE_LOCAL_INJECTORS_DISABLED", "workspace local injectors are disabled by tenant policy")
		}
	}

	var req request.UpdateInjectorRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	fieldErrors := validateInjectorSchemaRequest(req)
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	now := time.Now().UTC()
	def := &domain.InjectorDefinition{
		Name:        req.Name,
		Description: req.Description,
		UpdatedAt:   now,
	}

	fields := make([]*domain.InjectorField, len(req.Fields))
	for i, f := range req.Fields {
		allowOverwrite := true
		if f.AllowOverwrite != nil {
			allowOverwrite = *f.AllowOverwrite
		}
		fields[i] = &domain.InjectorField{
			ID:             uuid.Must(uuid.NewV7()),
			FieldName:      f.FieldName,
			FieldType:      domain.InjectorFieldType(f.FieldType),
			Description:    f.Description,
			Position:       f.Position,
			DefaultValue:   f.DefaultValue,
			AllowOverwrite: allowOverwrite,
		}
	}

	if err := h.store.UpdateDefinitionSchema(c.Request().Context(), currentName, workspaceID, def, fields); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewInjectorCreateResponse(def, fields))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/injectors.
func (h *InjectorHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.list(c, ws)
}

// ListGlobal handles GET /global/injectors.
func (h *InjectorHandler) ListGlobal(c *echo.Context) error {
	return h.list(c, nil)
}

func (h *InjectorHandler) list(c *echo.Context, workspace *domain.Workspace) error {
	ctx := c.Request().Context()
	systemWorkspace, err := resolveSystemWorkspace(ctx, workspace, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	chain, err := h.listChain(ctx, workspace, includeInheritedInjectors(c))
	if err != nil {
		return mapStoreError(c, err)
	}

	defs, err := h.store.ListDefinitionsInChain(ctx, chain)
	if err != nil {
		return mapStoreError(c, err)
	}
	defs = dedupeDefinitionsByPriority(defs, chain)

	defIDs := make([]uuid.UUID, 0, len(defs))
	for _, def := range defs {
		defIDs = append(defIDs, def.ID)
	}

	fieldsByDefinition, err := h.store.GetAllFieldsByDefinitions(ctx, defIDs)
	if err != nil {
		return mapStoreError(c, err)
	}
	for _, def := range defs {
		annotateInjectorScope(def, workspace, systemWorkspace)
	}

	return c.JSON(http.StatusOK, response.NewInjectorListResponse(defs, fieldsByDefinition))
}

func (h *InjectorHandler) listChain(ctx context.Context, workspace *domain.Workspace, includeInherited bool) ([]uuid.NullUUID, error) {
	if workspace == nil {
		return []uuid.NullUUID{{}}, nil
	}

	chain := []uuid.NullUUID{uuidToNullUUID(&workspace.ID)}
	if !includeInherited {
		return chain, nil
	}

	systemWorkspace, err := h.wsStore.GetSystemWorkspace(ctx, workspace.TenantID, workspace.Environment)
	if err != nil {
		if apperr.IsNotFound(err) || errorsIsNotFound(err) {
			return chain, nil
		}
		return nil, err
	}
	if systemWorkspace != nil && systemWorkspace.ID != workspace.ID {
		chain = append(chain, uuidToNullUUID(&systemWorkspace.ID))
	}

	return chain, nil
}

func includeInheritedInjectors(c *echo.Context) bool {
	return strings.EqualFold(c.QueryParam("include_inherited"), "true")
}

func dedupeDefinitionsByPriority(defs []*domain.InjectorDefinition, chain []uuid.NullUUID) []*domain.InjectorDefinition {
	type entry struct {
		index int
		def   *domain.InjectorDefinition
	}

	bestByName := make(map[string]entry, len(defs))
	for _, def := range defs {
		scopeIdx := handlerScopeIndex(def.WorkspaceID, chain)
		existing, found := bestByName[def.Name]
		if !found || scopeIdx < existing.index {
			bestByName[def.Name] = entry{index: scopeIdx, def: def}
		}
	}

	result := make([]*domain.InjectorDefinition, 0, len(bestByName))
	seen := make(map[string]struct{}, len(bestByName))
	for _, def := range defs {
		best := bestByName[def.Name]
		if best.def != def {
			continue
		}
		if _, ok := seen[def.Name]; ok {
			continue
		}
		result = append(result, def)
		seen[def.Name] = struct{}{}
	}

	return result
}

func handlerScopeIndex(workspaceID *uuid.UUID, chain []uuid.NullUUID) int {
	for i, scope := range chain {
		if workspaceID == nil && !scope.Valid {
			return i
		}
		if workspaceID != nil && scope.Valid && *workspaceID == scope.UUID {
			return i
		}
	}
	return len(chain)
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/injectors/:name.
func (h *InjectorHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.get(c, ws)
}

// GetGlobal handles GET /global/injectors/:name.
func (h *InjectorHandler) GetGlobal(c *echo.Context) error {
	return h.get(c, nil)
}

func (h *InjectorHandler) get(c *echo.Context, workspace *domain.Workspace) error {
	name := c.Param("name")
	ctx := c.Request().Context()

	var (
		def             *domain.InjectorDefinition
		systemWorkspace *domain.Workspace
		err             error
		chain           []uuid.NullUUID
	)
	if workspace == nil {
		def, err = h.store.FindDefinitionByName(ctx, name, nil)
		chain = []uuid.NullUUID{uuid.NullUUID{}}
	} else {
		def, systemWorkspace, _, err = h.loadWorkspaceDefinitionAccess(ctx, workspace, name)
		if err == nil {
			chain, _, err = workspaceResolutionChain(ctx, workspace, h.wsStore)
		}
	}
	if err != nil {
		return mapStoreError(c, err)
	}
	annotateInjectorScope(def, workspace, systemWorkspace)

	fields, err := h.store.GetFieldsByDefinition(ctx, def.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	values, err := h.store.GetValues(ctx, def.ID, chain)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewInjectorDetailResponse(def, fields, values))
}

// Delete handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/injectors/:name.
func (h *InjectorHandler) Delete(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.deleteInjector(c, ws)
}

// DeleteGlobal handles DELETE /global/injectors/:name.
func (h *InjectorHandler) DeleteGlobal(c *echo.Context) error {
	return h.deleteInjector(c, nil)
}

func (h *InjectorHandler) deleteInjector(c *echo.Context, workspace *domain.Workspace) error {
	name := c.Param("name")
	if workspace == nil {
		def, err := h.store.FindDefinitionByName(c.Request().Context(), name, nil)
		if err != nil {
			return mapStoreError(c, err)
		}
		if err := h.store.SoftDeleteDefinition(c.Request().Context(), def.ID); err != nil {
			return mapStoreError(c, err)
		}
		return c.NoContent(http.StatusNoContent)
	}

	def, _, policies, err := h.loadWorkspaceDefinitionAccess(c.Request().Context(), workspace, name)
	if err != nil {
		return mapStoreError(c, err)
	}
	if !isOwnedByCurrentWorkspace(def.WorkspaceID, workspace) {
		return writePolicyForbidden(c, "READ_ONLY_INHERITED_INJECTOR", "inherited injectors are read-only in workspace scope")
	}
	if !workspace.IsSystem && !policies.AllowWorkspaceLocalInjectors {
		return writePolicyForbidden(c, "WORKSPACE_LOCAL_INJECTORS_DISABLED", "workspace local injectors are disabled by tenant policy")
	}
	if err := h.store.SoftDeleteDefinition(c.Request().Context(), def.ID); err != nil {
		return mapStoreError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// SetValues handles PUT /tenants/:tenant_code/workspaces/:workspace_code/injectors/:name/values.
func (h *InjectorHandler) SetValues(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.setValues(c, &ws.ID)
}

// UpdateField handles PUT /tenants/:tenant_code/workspaces/:workspace_code/injectors/:name/fields/:field_name.
func (h *InjectorHandler) UpdateField(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.updateField(c, ws)
}

// UpdateFieldGlobal handles PUT /global/injectors/:name/fields/:field_name.
func (h *InjectorHandler) UpdateFieldGlobal(c *echo.Context) error {
	return h.updateField(c, nil)
}

func (h *InjectorHandler) updateField(c *echo.Context, workspace *domain.Workspace) error {
	name := c.Param("name")
	fieldName := c.Param("field_name")
	ctx := c.Request().Context()

	var workspaceID *uuid.UUID
	if workspace != nil {
		workspaceID = &workspace.ID
		def, _, policies, err := h.loadWorkspaceDefinitionAccess(ctx, workspace, name)
		if err != nil {
			return mapStoreError(c, err)
		}
		if !isOwnedByCurrentWorkspace(def.WorkspaceID, workspace) {
			return writePolicyForbidden(c, "READ_ONLY_INHERITED_INJECTOR", "inherited injectors are read-only in workspace scope")
		}
		if !workspace.IsSystem && !policies.AllowWorkspaceLocalInjectors {
			return writePolicyForbidden(c, "WORKSPACE_LOCAL_INJECTORS_DISABLED", "workspace local injectors are disabled by tenant policy")
		}
	}

	def, err := h.store.FindDefinitionByName(ctx, name, workspaceID)
	if err != nil {
		return mapStoreError(c, err)
	}

	fields, err := h.store.GetFieldsByDefinition(ctx, def.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	field := findField(fields, fieldName)
	if field == nil {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	var req request.UpdateInjectorFieldRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	allowOverwrite := field.AllowOverwrite
	if req.AllowOverwrite != nil {
		allowOverwrite = *req.AllowOverwrite
	}

	defaultValue := field.DefaultValue
	if req.DefaultValue != nil {
		defaultValue = req.DefaultValue
	}

	if err := validateFieldDefault(field.FieldType, defaultValue, allowOverwrite); err != nil {
		return response.WriteError(
			c,
			http.StatusUnprocessableEntity,
			"VALIDATION_ERROR",
			"validation failed",
			response.FieldError{Field: "default_value", Message: err.Error()},
		)
	}

	field.DefaultValue = defaultValue
	field.AllowOverwrite = allowOverwrite

	if err := h.store.UpdateField(ctx, field); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewInjectorFieldResponse(field))
}

func (h *InjectorHandler) setValues(c *echo.Context, workspaceID *uuid.UUID) error {
	name := c.Param("name")
	ctx := c.Request().Context()

	if workspaceID != nil {
		workspace, err := resolveWorkspace(c, h.tsStore, h.wsStore)
		if err != nil {
			return mapStoreError(c, err)
		}
		def, _, policies, err := h.loadWorkspaceDefinitionAccess(ctx, workspace, name)
		if err != nil {
			return mapStoreError(c, err)
		}
		if !isOwnedByCurrentWorkspace(def.WorkspaceID, workspace) {
			return writePolicyForbidden(c, "READ_ONLY_INHERITED_INJECTOR", "inherited injectors are read-only in workspace scope")
		}
		if !workspace.IsSystem && !policies.AllowWorkspaceLocalInjectors {
			return writePolicyForbidden(c, "WORKSPACE_LOCAL_INJECTORS_DISABLED", "workspace local injectors are disabled by tenant policy")
		}
	}

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

func (h *InjectorHandler) loadWorkspaceDefinitionAccess(
	ctx context.Context,
	workspace *domain.Workspace,
	name string,
) (*domain.InjectorDefinition, *domain.Workspace, workspacePolicies, error) {
	systemWorkspace, err := resolveSystemWorkspace(ctx, workspace, h.wsStore)
	if err != nil {
		return nil, nil, workspacePolicies{}, err
	}

	def, err := h.store.FindDefinitionByName(ctx, name, &workspace.ID)
	if err != nil {
		if !apperr.IsNotFound(err) && !errorsIsNotFound(err) {
			return nil, nil, workspacePolicies{}, err
		}
		def = nil
	}
	if def == nil && systemWorkspace != nil && systemWorkspace.ID != workspace.ID {
		def, err = h.store.FindDefinitionByName(ctx, name, &systemWorkspace.ID)
		if err != nil {
			if !apperr.IsNotFound(err) && !errorsIsNotFound(err) {
				return nil, nil, workspacePolicies{}, err
			}
			def = nil
		}
	}
	if def == nil {
		return nil, systemWorkspace, workspacePolicies{}, apperr.NotFound("injector not found in workspace scope")
	}
	annotateInjectorScope(def, workspace, systemWorkspace)
	return def, systemWorkspace, effectiveWorkspacePolicies(systemWorkspace), nil
}

func errorsIsNotFound(err error) bool {
	return err != nil && (apperr.IsNotFound(err) || errors.Is(err, domain.ErrNotFound))
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

func findField(fields []*domain.InjectorField, name string) *domain.InjectorField {
	for _, field := range fields {
		if field.FieldName == name {
			return field
		}
	}
	return nil
}

func validateFieldDefault(fieldType domain.InjectorFieldType, defaultValue any, allowOverwrite bool) error {
	if !allowOverwrite {
		if isEmptyDefault(defaultValue) {
			return domainValidationError("default_value is required when allow_overwrite is false")
		}
	}

	if defaultValue == nil {
		return nil
	}

	switch fieldType {
	case domain.FieldTypeText, domain.FieldTypeImg, domain.FieldTypeURL, domain.FieldTypeHTML:
		if _, ok := defaultValue.(string); !ok {
			return domainValidationError("default_value must be a string for field type " + string(fieldType))
		}
	case domain.FieldTypeNumber:
		switch defaultValue.(type) {
		case int, int8, int16, int32, int64, float32, float64:
			return nil
		default:
			return domainValidationError("default_value must be a number for field type number")
		}
	case domain.FieldTypeBool:
		if _, ok := defaultValue.(bool); !ok {
			return domainValidationError("default_value must be a boolean for field type bool")
		}
	}

	return nil
}

func validateInjectorSchemaRequest(req request.CreateInjectorRequest) []response.FieldError {
	fieldErrors := make([]response.FieldError, 0)
	seenFieldNames := make(map[string]int, len(req.Fields))

	if strings.TrimSpace(req.Name) == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	}
	if len(req.Fields) == 0 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "fields", Message: "at least one field is required"})
	}

	for i, f := range req.Fields {
		allowOverwrite := true
		if f.AllowOverwrite != nil {
			allowOverwrite = *f.AllowOverwrite
		}

		trimmedFieldName := strings.TrimSpace(f.FieldName)
		if trimmedFieldName == "" {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "fields", Message: "field_name is required at index " + itoa(i)})
		} else {
			seenFieldNames[trimmedFieldName]++
			if seenFieldNames[trimmedFieldName] > 1 {
				fieldErrors = append(fieldErrors, response.FieldError{Field: "fields", Message: "duplicate field_name at index " + itoa(i)})
			}
		}

		if !isValidFieldType(f.FieldType) {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "fields", Message: "invalid field_type at index " + itoa(i)})
		}
		if err := validateFieldDefault(domain.InjectorFieldType(f.FieldType), f.DefaultValue, allowOverwrite); err != nil {
			fieldErrors = append(fieldErrors, response.FieldError{
				Field:   "fields",
				Message: err.Error() + " at index " + itoa(i),
			})
		}
	}

	return fieldErrors
}

func isEmptyDefault(v any) bool {
	if v == nil {
		return true
	}

	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}

	return false
}

func domainValidationError(msg string) error {
	return &validationError{msg: msg}
}

type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }

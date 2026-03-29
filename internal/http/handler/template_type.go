package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/pkg/slug"
)

// TemplateTypeHandler handles CRUD operations for template types.
type TemplateTypeHandler struct {
	svc     *service.TemplateTypeService
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewTemplateTypeHandler creates a new TemplateTypeHandler.
func NewTemplateTypeHandler(svc *service.TemplateTypeService, ts port.TenantStore, ws port.WorkspaceStore) *TemplateTypeHandler {
	return &TemplateTypeHandler{svc: svc, tsStore: ts, wsStore: ws}
}

// Create handles POST /tenants/:tenant_code/workspaces/:workspace_code/template-types.
func (h *TemplateTypeHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.create(c, &ws.ID)
}

// CreateGlobal handles POST /global/template-types.
func (h *TemplateTypeHandler) CreateGlobal(c *echo.Context) error {
	return h.create(c, nil)
}

func (h *TemplateTypeHandler) create(c *echo.Context, workspaceID *uuid.UUID) error {
	var req request.CreateTemplateTypeRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if err := slug.Validate(req.Slug); err != nil {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "slug", Message: err.Error()})
	}
	if req.Name == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	} else if len(req.Name) > 255 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "must be at most 255 characters"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	var adapterID *uuid.UUID
	if req.AdapterID != nil {
		parsed, err := uuid.Parse(*req.AdapterID)
		if err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "adapter_id", Message: "must be a valid UUID"},
			)
		}
		adapterID = &parsed
	}

	var variableSchema map[string]any
	if len(req.VariableSchema) > 0 {
		if err := json.Unmarshal(req.VariableSchema, &variableSchema); err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "variable_schema", Message: "must be valid JSON"},
			)
		}
	}

	tt, err := h.svc.Create(c.Request().Context(), req.Slug, req.Name, req.Description, adapterID, variableSchema, workspaceID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewTemplateTypeResponse(tt))
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/template-types/:slug.
func (h *TemplateTypeHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.get(c, &ws.ID)
}

// GetGlobal handles GET /global/template-types/:slug.
func (h *TemplateTypeHandler) GetGlobal(c *echo.Context) error {
	return h.get(c, nil)
}

func (h *TemplateTypeHandler) get(c *echo.Context, workspaceID *uuid.UUID) error {
	slugParam := c.Param("slug")

	tt, err := h.svc.FindBySlugInScope(c.Request().Context(), slugParam, workspaceID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(tt.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	return c.JSON(http.StatusOK, response.NewTemplateTypeResponse(tt))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/template-types.
func (h *TemplateTypeHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.listTypes(c, &ws.ID)
}

// ListGlobal handles GET /global/template-types.
func (h *TemplateTypeHandler) ListGlobal(c *echo.Context) error {
	return h.listTypes(c, nil)
}

func (h *TemplateTypeHandler) listTypes(c *echo.Context, wsID *uuid.UUID) error {
	opts := pagination.ParseListOptions(c)

	types, nextCursor, err := h.svc.ListTypes(c.Request().Context(), wsID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	items := make([]response.TemplateTypeResponse, len(types))
	for i, tt := range types {
		items[i] = response.NewTemplateTypeResponse(tt)
	}

	return c.JSON(http.StatusOK, response.TemplateTypeListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

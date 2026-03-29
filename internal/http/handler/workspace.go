package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/slug"
)

// WorkspaceHandler handles CRUD operations for workspaces.
type WorkspaceHandler struct {
	tenantStore port.TenantStore
	wsStore     port.WorkspaceStore
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(ts port.TenantStore, ws port.WorkspaceStore) *WorkspaceHandler {
	return &WorkspaceHandler{tenantStore: ts, wsStore: ws}
}

// resolveTenant looks up a tenant by the :tenant_code path param.
func (h *WorkspaceHandler) resolveTenant(c *echo.Context) (*domain.Tenant, error) {
	code := c.Param("tenant_code")
	tenant, err := h.tenantStore.GetByCode(c.Request().Context(), code)
	if err != nil {
		return nil, err
	}
	return tenant, nil
}

// Create handles POST /api/v1/manage/tenants/:tenant_code/workspaces.
func (h *WorkspaceHandler) Create(c *echo.Context) error {
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.CreateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if err := slug.Validate(req.Code); err != nil {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "code", Message: err.Error()})
	}
	if req.Name == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	} else if len(req.Name) > 255 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "must be at most 255 characters"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	now := time.Now().UTC()
	ws := &domain.Workspace{
		ID:            uuid.Must(uuid.NewV7()),
		TenantID:      tenant.ID,
		Code:          req.Code,
		Name:          req.Name,
		DefaultLocale: req.DefaultLocale,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.wsStore.Create(c.Request().Context(), ws); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewWorkspaceResponse(ws))
}

// List handles GET /api/v1/manage/tenants/:tenant_code/workspaces.
func (h *WorkspaceHandler) List(c *echo.Context) error {
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	opts := pagination.ParseListOptions(c)

	workspaces, nextCursor, err := h.wsStore.ListByTenant(c.Request().Context(), tenant.ID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	items := make([]response.WorkspaceResponse, len(workspaces))
	for i, ws := range workspaces {
		items[i] = response.NewWorkspaceResponse(ws)
	}

	return c.JSON(http.StatusOK, response.WorkspaceListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Get handles GET /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code.
func (h *WorkspaceHandler) Get(c *echo.Context) error {
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	wsCode := c.Param("workspace_code")
	ws, err := h.wsStore.GetByTenantAndCode(c.Request().Context(), tenant.ID, wsCode)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewWorkspaceResponse(ws))
}

// Update handles PUT /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code.
func (h *WorkspaceHandler) Update(c *echo.Context) error {
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	wsCode := c.Param("workspace_code")
	ctx := c.Request().Context()
	ws, err := h.wsStore.GetByTenantAndCode(ctx, tenant.ID, wsCode)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.UpdateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.Name != nil {
		if *req.Name == "" {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "name", Message: "cannot be empty"},
			)
		}
		if len(*req.Name) > 255 {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "name", Message: "must be at most 255 characters"},
			)
		}
		ws.Name = *req.Name
	}
	if req.OpenTrackingEnabled != nil {
		ws.OpenTrackingEnabled = *req.OpenTrackingEnabled
	}
	if req.DefaultLocale != nil {
		ws.DefaultLocale = req.DefaultLocale
	}

	ws.UpdatedAt = time.Now().UTC()
	if err := h.wsStore.Update(ctx, ws); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewWorkspaceResponse(ws))
}

// SoftDelete handles DELETE /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code.
func (h *WorkspaceHandler) SoftDelete(c *echo.Context) error {
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	wsCode := c.Param("workspace_code")
	ctx := c.Request().Context()
	ws, err := h.wsStore.GetByTenantAndCode(ctx, tenant.ID, wsCode)
	if err != nil {
		return mapStoreError(c, err)
	}

	if err := h.wsStore.SoftDelete(ctx, ws.ID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

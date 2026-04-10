package handler

import (
	"net/http"
	"strings"
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
	emailStore  port.EmailStore
}

const systemWorkspaceProtectedMessage = "system workspace is protected and cannot be modified from workspace management"

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(ts port.TenantStore, ws port.WorkspaceStore, emailStore port.EmailStore) *WorkspaceHandler {
	return &WorkspaceHandler{tenantStore: ts, wsStore: ws, emailStore: emailStore}
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
	logicalWorkspaceID := uuid.Must(uuid.NewV7())
	prod := &domain.Workspace{
		ID:                 uuid.Must(uuid.NewV7()),
		LogicalWorkspaceID: logicalWorkspaceID,
		TenantID:           tenant.ID,
		Code:               req.Code,
		Name:               req.Name,
		Environment:        domain.EnvironmentProd,
		IsActive:           true,
		DefaultLocale:      req.DefaultLocale,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	test := &domain.Workspace{
		ID:                 uuid.Must(uuid.NewV7()),
		LogicalWorkspaceID: logicalWorkspaceID,
		TenantID:           tenant.ID,
		Code:               req.Code,
		Name:               req.Name,
		Environment:        domain.EnvironmentTest,
		IsActive:           true,
		DefaultLocale:      req.DefaultLocale,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := h.wsStore.CreateLogicalPair(c.Request().Context(), prod, test); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewWorkspaceResponse(prod))
}

// List handles GET /api/v1/manage/tenants/:tenant_code/workspaces.
func (h *WorkspaceHandler) List(c *echo.Context) error {
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	opts := pagination.ParseListOptions(c)

	workspaces, nextCursor, err := h.wsStore.ListByTenant(c.Request().Context(), tenant.ID, requestEnvironment(c), opts)
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
	ws, err := h.wsStore.GetByTenantAndCode(c.Request().Context(), tenant.ID, wsCode, requestEnvironment(c))
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewWorkspaceResponse(ws))
}

// Update handles PUT /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code.
func (h *WorkspaceHandler) Update(c *echo.Context) error { //nolint:gocognit,gocyclo,funlen // shared and environment-scoped workspace updates intentionally share validation/orchestration
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	wsCode := c.Param("workspace_code")
	ctx := c.Request().Context()
	environment := requestEnvironment(c)
	ws, err := h.wsStore.GetByTenantAndCode(ctx, tenant.ID, wsCode, environment)
	if err != nil {
		return mapStoreError(c, err)
	}
	if ws.IsSystem {
		return response.WriteError(c, http.StatusConflict, "SYSTEM_WORKSPACE_PROTECTED", systemWorkspaceProtectedMessage)
	}

	var req request.UpdateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	isEnvironmentScoped := c.Param("environment") != ""
	if !isEnvironmentScoped {
		fieldErrors := make([]response.FieldError, 0, 5)
		if req.IsActive != nil {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "is_active", Message: "is only supported on environment-scoped workspace routes"})
		}
		if req.OpenTrackingEnabled != nil {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "open_tracking_enabled", Message: "is only supported on environment-scoped workspace routes"})
		}
		if req.DefaultLocale != nil {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "default_locale", Message: "is only supported on environment-scoped workspace routes"})
		}
		if req.TestRecipientMode != nil {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "test_recipient_mode", Message: "is only supported on environment-scoped workspace routes"})
		}
		if req.TestRecipientAddresses != nil {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "test_recipient_addresses", Message: "is only supported on environment-scoped workspace routes"})
		}
		if len(fieldErrors) > 0 {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
		}
	}

	nextCode := ws.Code
	if req.Code != nil {
		if err := slug.Validate(*req.Code); err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "code", Message: err.Error()},
			)
		}
		nextCode = *req.Code
	}

	nextName := ws.Name
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
		nextName = *req.Name
	}

	if nextCode != ws.Code || nextName != ws.Name {
		if err := h.wsStore.UpdateShared(ctx, tenant.ID, ws.Code, nextCode, nextName); err != nil {
			return mapStoreError(c, err)
		}
		ws.Code = nextCode
		ws.Name = nextName
	}

	needsEnvironmentUpdate := false
	if req.IsActive != nil {
		ws.IsActive = *req.IsActive
		needsEnvironmentUpdate = true
	}
	if req.OpenTrackingEnabled != nil {
		ws.OpenTrackingEnabled = *req.OpenTrackingEnabled
		needsEnvironmentUpdate = true
	}
	if req.DefaultLocale != nil {
		ws.DefaultLocale = req.DefaultLocale
		needsEnvironmentUpdate = true
	}
	if req.TestRecipientMode != nil || req.TestRecipientAddresses != nil {
		if environment != domain.EnvironmentTest {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "test_recipient_mode", Message: "is only supported in the test environment"},
			)
		}

		nextMode := ws.TestRecipientMode
		if req.TestRecipientMode != nil {
			nextMode = domain.TestRecipientMode(strings.TrimSpace(*req.TestRecipientMode))
			if !nextMode.Valid() {
				return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
					response.FieldError{Field: "test_recipient_mode", Message: "must be replace or append"},
				)
			}
		}

		nextAddresses := append([]string(nil), ws.TestRecipientAddresses...)
		if req.TestRecipientAddresses != nil {
			nextAddresses = domain.NormalizeRecipientAddresses(*req.TestRecipientAddresses)
			if err := domain.ValidateRecipientAddresses(nextAddresses); err != nil {
				return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
					response.FieldError{Field: "test_recipient_addresses", Message: "must contain valid email addresses"},
				)
			}
		}

		ws.TestRecipientMode = nextMode
		ws.TestRecipientAddresses = nextAddresses
		needsEnvironmentUpdate = true
	}

	if needsEnvironmentUpdate {
		ws.UpdatedAt = time.Now().UTC()
		if err := h.wsStore.Update(ctx, ws); err != nil {
			return mapStoreError(c, err)
		}
	}

	refreshed, err := h.wsStore.GetByTenantAndCode(ctx, tenant.ID, ws.Code, environment)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewWorkspaceResponse(refreshed))
}

// SoftDelete handles DELETE /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code.
func (h *WorkspaceHandler) SoftDelete(c *echo.Context) error {
	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	wsCode := c.Param("workspace_code")
	ctx := c.Request().Context()
	ws, err := h.wsStore.GetByTenantAndCode(ctx, tenant.ID, wsCode, requestEnvironment(c))
	if err != nil {
		return mapStoreError(c, err)
	}
	if ws.IsSystem {
		return response.WriteError(c, http.StatusConflict, "SYSTEM_WORKSPACE_PROTECTED", systemWorkspaceProtectedMessage)
	}

	if err := h.wsStore.SoftDeleteLogical(ctx, tenant.ID, ws.Code); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// ResetRuntime handles POST /api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/runtime/reset.
func (h *WorkspaceHandler) ResetRuntime(c *echo.Context) error {
	if requestEnvironment(c) != domain.EnvironmentTest {
		return response.WriteError(c, http.StatusConflict, "TEST_ENVIRONMENT_REQUIRED", "runtime reset is only available for the test environment")
	}

	tenant, err := h.resolveTenant(c)
	if err != nil {
		return mapStoreError(c, err)
	}

	wsCode := c.Param("workspace_code")
	ctx := c.Request().Context()
	ws, err := h.wsStore.GetByTenantAndCode(ctx, tenant.ID, wsCode, domain.EnvironmentTest)
	if err != nil {
		return mapStoreError(c, err)
	}

	if err := h.emailStore.PurgeWorkspaceRuntime(ctx, ws.ID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

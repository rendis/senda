package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
	"github.com/rendis/senda/pkg/slug"
)

// TenantHandler handles CRUD operations for tenants.
type TenantHandler struct {
	store        port.TenantStore
	wsStore      port.WorkspaceStore
	adapterStore port.AdapterStore
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler(ts port.TenantStore, ws port.WorkspaceStore, as port.AdapterStore) *TenantHandler {
	return &TenantHandler{store: ts, wsStore: ws, adapterStore: as}
}

// Create handles POST /api/v1/manage/tenants.
func (h *TenantHandler) Create(c *echo.Context) error {
	var req request.CreateTenantRequest
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
	tenant := &domain.Tenant{
		ID:        uuid.Must(uuid.NewV7()),
		Code:      req.Code,
		Name:      req.Name,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx := c.Request().Context()
	if err := h.store.Create(ctx, tenant); err != nil {
		return mapStoreError(c, err)
	}

	// Create the _system workspace for the new tenant.
	sysWS := &domain.Workspace{
		ID:        uuid.Must(uuid.NewV7()),
		TenantID:  tenant.ID,
		Code:      "_system",
		Name:      "System",
		IsSystem:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.wsStore.Create(ctx, sysWS); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewTenantResponse(tenant))
}

// List handles GET /api/v1/manage/tenants.
func (h *TenantHandler) List(c *echo.Context) error {
	opts := pagination.ParseListOptions(c)

	tenants, nextCursor, err := h.store.List(c.Request().Context(), opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	items := make([]response.TenantResponse, len(tenants))
	for i, t := range tenants {
		resp := response.NewTenantResponse(t)
		blockedReason, err := h.deleteBlockedReason(c.Request().Context(), t.ID)
		if err != nil {
			return mapStoreError(c, err)
		}
		resp.DeleteBlockedReason = blockedReason
		items[i] = resp
	}

	return c.JSON(http.StatusOK, response.TenantListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// GetByCode handles GET /api/v1/manage/tenants/:tenant_code.
func (h *TenantHandler) GetByCode(c *echo.Context) error {
	code := c.Param("tenant_code")

	tenant, err := h.store.GetByCode(c.Request().Context(), code)
	if err != nil {
		return mapStoreError(c, err)
	}

	resp := response.NewTenantResponse(tenant)
	blockedReason, err := h.deleteBlockedReason(c.Request().Context(), tenant.ID)
	if err != nil {
		return mapStoreError(c, err)
	}
	resp.DeleteBlockedReason = blockedReason
	return c.JSON(http.StatusOK, resp)
}

// Update handles PUT /api/v1/manage/tenants/:tenant_code.
func (h *TenantHandler) Update(c *echo.Context) error {
	code := c.Param("tenant_code")

	ctx := c.Request().Context()
	tenant, err := h.store.GetByCode(ctx, code)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.UpdateTenantRequest
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
		tenant.Name = *req.Name
	}
	if req.IsActive != nil {
		tenant.IsActive = *req.IsActive
	}

	tenant.UpdatedAt = time.Now().UTC()
	if err := h.store.Update(ctx, tenant); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewTenantResponse(tenant))
}

// SoftDelete handles DELETE /api/v1/manage/tenants/:tenant_code.
func (h *TenantHandler) SoftDelete(c *echo.Context) error {
	code := c.Param("tenant_code")

	ctx := c.Request().Context()
	tenant, err := h.store.GetByCode(ctx, code)
	if err != nil {
		return mapStoreError(c, err)
	}

	if err := h.store.SoftDelete(ctx, tenant.ID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

const tenantDeleteBlockedReasonTemplate = "Delete disabled: tenant still has active SES adapter %q in workspace %q. Delete it first."

func (h *TenantHandler) deleteBlockedReason(ctx context.Context, tenantID uuid.UUID) (string, error) {
	if h.adapterStore == nil {
		return "", nil
	}

	workspaceCursor := ""
	for {
		workspaces, nextCursor, err := h.wsStore.ListByTenant(ctx, tenantID, port.ListOptions{
			Cursor: workspaceCursor,
			Limit:  100,
		})
		if err != nil {
			return "", err
		}

		for _, ws := range workspaces {
			adapterCursor := ""
			for {
				page, err := h.adapterStore.ListByWorkspace(ctx, &ws.ID, port.ListOptions{
					Cursor: adapterCursor,
					Limit:  100,
				})
				if err != nil {
					return "", err
				}

				for _, adapter := range page.Items {
					if adapter.AdapterType == domain.AdapterTypeSES {
						return responseMessageDeleteBlocked(adapter.Name, ws.Code), nil
					}
				}

				if !page.HasMore || page.NextCursor == "" {
					break
				}
				adapterCursor = page.NextCursor
			}
		}

		if nextCursor == "" {
			break
		}
		workspaceCursor = nextCursor
	}

	return "", nil
}

func responseMessageDeleteBlocked(adapterName, workspaceCode string) string {
	return fmt.Sprintf(tenantDeleteBlockedReasonTemplate, adapterName, workspaceCode)
}

// mapStoreError maps domain errors to HTTP error responses.
func mapStoreError(c *echo.Context, err error) error {
	// Check domain sentinel errors first.
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		return response.WriteError(c, http.StatusConflict, "CONFLICT", "resource already exists")
	case errors.Is(err, domain.ErrValidation):
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, domain.ErrInvalidCursor):
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid cursor")
	}

	// Check apperr.AppError (used by repos for typed errors with HTTP status).
	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		code := "INTERNAL_ERROR"
		switch appErr.Code {
		case http.StatusNotFound:
			code = "NOT_FOUND"
		case http.StatusConflict:
			code = "CONFLICT"
		case http.StatusBadRequest:
			code = "BAD_REQUEST"
		case http.StatusUnprocessableEntity:
			code = "VALIDATION_ERROR"
		case http.StatusForbidden:
			code = "FORBIDDEN"
		}
		return response.WriteError(c, appErr.Code, code, appErr.Message)
	}

	slog.ErrorContext(c.Request().Context(), "unhandled store error",
		slog.String("error", err.Error()),
		slog.String("path", c.Request().URL.Path),
	)
	return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}

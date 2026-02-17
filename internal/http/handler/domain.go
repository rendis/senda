package handler

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/http/pagination"
	"github.com/senda-app/senda/internal/http/request"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// DomainHTTPHandler handles CRUD operations for domain records.
// Named DomainHTTPHandler to avoid collision with the domain package.
type DomainHTTPHandler struct {
	svc     *service.DomainService
	store   port.DomainStore
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewDomainHTTPHandler creates a new DomainHTTPHandler.
func NewDomainHTTPHandler(svc *service.DomainService, ds port.DomainStore, ts port.TenantStore, ws port.WorkspaceStore) *DomainHTTPHandler {
	return &DomainHTTPHandler{svc: svc, store: ds, tsStore: ts, wsStore: ws}
}

// Register handles POST /tenants/:tenant_code/workspaces/:workspace_code/domains.
func (h *DomainHTTPHandler) Register(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.register(c, &ws.ID)
}

// RegisterGlobal handles POST /global/domains.
func (h *DomainHTTPHandler) RegisterGlobal(c *echo.Context) error {
	return h.register(c, nil)
}

func (h *DomainHTTPHandler) register(c *echo.Context, workspaceID *uuid.UUID) error {
	var req request.RegisterDomainRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.DomainName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "domain_name", Message: "is required"})
	} else if !isValidDomainName(req.DomainName) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "domain_name", Message: "must be a valid domain name"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	d, err := h.svc.Register(c.Request().Context(), req.DomainName, workspaceID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewDomainResponse(d))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/domains.
func (h *DomainHTTPHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.list(c, &ws.ID)
}

// ListGlobal handles GET /global/domains.
func (h *DomainHTTPHandler) ListGlobal(c *echo.Context) error {
	return h.list(c, nil)
}

func (h *DomainHTTPHandler) list(c *echo.Context, workspaceID *uuid.UUID) error {
	opts := pagination.ParseListOptions(c)

	page, err := h.store.ListByWorkspace(c.Request().Context(), workspaceID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewDomainListResponse(page))
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/domains/:id.
func (h *DomainHTTPHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.get(c, &ws.ID)
}

// GetGlobal handles GET /global/domains/:id.
func (h *DomainHTTPHandler) GetGlobal(c *echo.Context) error {
	return h.get(c, nil)
}

func (h *DomainHTTPHandler) get(c *echo.Context, workspaceID *uuid.UUID) error {
	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid domain ID")
	}

	d, err := h.store.GetByID(c.Request().Context(), domainID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(d.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	return c.JSON(http.StatusOK, response.NewDomainResponse(d))
}

// VerifyNow handles POST /tenants/:tenant_code/workspaces/:workspace_code/domains/:id/verify.
func (h *DomainHTTPHandler) VerifyNow(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid domain ID")
	}

	ctx := c.Request().Context()
	d, err := h.store.GetByID(ctx, domainID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(d.WorkspaceID, &ws.ID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	if err := h.svc.RequestVerification(ctx, domainID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusAccepted)
}

// SoftDelete handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/domains/:id.
func (h *DomainHTTPHandler) SoftDelete(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.softDelete(c, &ws.ID)
}

// SoftDeleteGlobal handles DELETE /global/domains/:id.
func (h *DomainHTTPHandler) SoftDeleteGlobal(c *echo.Context) error {
	return h.softDelete(c, nil)
}

func (h *DomainHTTPHandler) softDelete(c *echo.Context, workspaceID *uuid.UUID) error {
	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid domain ID")
	}

	ctx := c.Request().Context()
	d, err := h.store.GetByID(ctx, domainID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(d.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	if err := h.store.SoftDelete(ctx, domainID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// isValidDomainName performs basic domain name validation.
func isValidDomainName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
	}
	return true
}

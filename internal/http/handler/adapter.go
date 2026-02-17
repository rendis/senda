package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/pagination"
	"github.com/senda-app/senda/internal/http/request"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

// AdapterHandler handles CRUD operations for email adapters.
type AdapterHandler struct {
	store   port.AdapterStore
	crypto  port.Crypto
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewAdapterHandler creates a new AdapterHandler.
func NewAdapterHandler(as port.AdapterStore, crypto port.Crypto, ts port.TenantStore, ws port.WorkspaceStore) *AdapterHandler {
	return &AdapterHandler{store: as, crypto: crypto, tsStore: ts, wsStore: ws}
}

// Create handles POST /tenants/:tenant_code/workspaces/:workspace_code/adapters.
func (h *AdapterHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.create(c, &ws.ID)
}

// CreateGlobal handles POST /global/adapters.
func (h *AdapterHandler) CreateGlobal(c *echo.Context) error {
	return h.create(c, nil)
}

func (h *AdapterHandler) create(c *echo.Context, workspaceID *uuid.UUID) error {
	var req request.CreateAdapterRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.Name == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	}
	if !isValidAdapterType(req.AdapterType) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "adapter_type", Message: "must be one of: ses, gmail"})
	}
	if len(req.Config) == 0 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "config", Message: "is required"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	encrypted, err := h.crypto.Encrypt(req.Config)
	if err != nil {
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}

	now := time.Now().UTC()
	adapter := &domain.Adapter{
		ID:                 uuid.Must(uuid.NewV7()),
		WorkspaceID:        workspaceID,
		Name:               req.Name,
		AdapterType:        domain.AdapterType(req.AdapterType),
		ConfigEncrypted:    encrypted,
		IsDefault:          req.IsDefault,
		RateLimitPerSecond: req.RateLimitPerSecond,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := h.store.Create(c.Request().Context(), adapter); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewAdapterResponse(adapter))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/adapters.
func (h *AdapterHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.list(c, &ws.ID)
}

// ListGlobal handles GET /global/adapters.
func (h *AdapterHandler) ListGlobal(c *echo.Context) error {
	return h.list(c, nil)
}

func (h *AdapterHandler) list(c *echo.Context, workspaceID *uuid.UUID) error {
	opts := pagination.ParseListOptions(c)

	page, err := h.store.ListByWorkspace(c.Request().Context(), workspaceID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterListResponse(page))
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/adapters/:id.
func (h *AdapterHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.get(c, &ws.ID)
}

// GetGlobal handles GET /global/adapters/:id.
func (h *AdapterHandler) GetGlobal(c *echo.Context) error {
	return h.get(c, nil)
}

func (h *AdapterHandler) get(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	adapter, err := h.store.GetByID(c.Request().Context(), adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	// Verify workspace ownership.
	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	return c.JSON(http.StatusOK, response.NewAdapterResponse(adapter))
}

// Update handles PUT /tenants/:tenant_code/workspaces/:workspace_code/adapters/:id.
func (h *AdapterHandler) Update(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.update(c, &ws.ID)
}

// UpdateGlobal handles PUT /global/adapters/:id.
func (h *AdapterHandler) UpdateGlobal(c *echo.Context) error {
	return h.update(c, nil)
}

func (h *AdapterHandler) update(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	ctx := c.Request().Context()
	adapter, err := h.store.GetByID(ctx, adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	var req request.UpdateAdapterRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.Name != nil {
		if *req.Name == "" {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "name", Message: "cannot be empty"},
			)
		}
		adapter.Name = *req.Name
	}
	if req.Config != nil {
		encrypted, err := h.crypto.Encrypt(*req.Config)
		if err != nil {
			return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		adapter.ConfigEncrypted = encrypted
	}
	if req.IsDefault != nil {
		adapter.IsDefault = *req.IsDefault
	}
	if req.RateLimitPerSecond != nil {
		adapter.RateLimitPerSecond = *req.RateLimitPerSecond
	}

	adapter.UpdatedAt = time.Now().UTC()
	if err := h.store.Update(ctx, adapter); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterResponse(adapter))
}

// SoftDelete handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/adapters/:id.
func (h *AdapterHandler) SoftDelete(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.softDelete(c, &ws.ID)
}

// SoftDeleteGlobal handles DELETE /global/adapters/:id.
func (h *AdapterHandler) SoftDeleteGlobal(c *echo.Context) error {
	return h.softDelete(c, nil)
}

func (h *AdapterHandler) softDelete(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	ctx := c.Request().Context()
	adapter, err := h.store.GetByID(ctx, adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	if err := h.store.SoftDelete(ctx, adapterID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func isValidAdapterType(t string) bool {
	switch domain.AdapterType(t) {
	case domain.AdapterTypeSES, domain.AdapterTypeGmail:
		return true
	}
	return false
}

// sameScope checks that both pointers are nil or point to the same UUID.
func sameScope(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

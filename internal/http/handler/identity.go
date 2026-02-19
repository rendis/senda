package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/request"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// IdentityHandler handles adapter identity management endpoints.
type IdentityHandler struct {
	svc     *service.IdentityService
	store   port.AdapterIdentityStore
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewIdentityHandler creates a new IdentityHandler.
func NewIdentityHandler(svc *service.IdentityService, store port.AdapterIdentityStore, ts port.TenantStore, ws port.WorkspaceStore) *IdentityHandler {
	return &IdentityHandler{svc: svc, store: store, tsStore: ts, wsStore: ws}
}

// List handles GET .../adapters/:id/identities (workspace scope).
func (h *IdentityHandler) List(c *echo.Context) error {
	return h.list(c)
}

// ListGlobal handles GET /global/adapters/:id/identities.
func (h *IdentityHandler) ListGlobal(c *echo.Context) error {
	return h.list(c)
}

func (h *IdentityHandler) list(c *echo.Context) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	identities, err := h.svc.ListIdentities(c.Request().Context(), adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterIdentityListResponse(identities))
}

// Sync handles POST .../adapters/:id/identities/sync (workspace scope).
func (h *IdentityHandler) Sync(c *echo.Context) error {
	return h.sync(c)
}

// SyncGlobal handles POST /global/adapters/:id/identities/sync.
func (h *IdentityHandler) SyncGlobal(c *echo.Context) error {
	return h.sync(c)
}

func (h *IdentityHandler) sync(c *echo.Context) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	identities, err := h.svc.SyncIdentities(c.Request().Context(), adapterID)
	if err != nil {
		return mapIdentityError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterIdentityListResponse(identities))
}

// Create handles POST .../adapters/:id/identities (workspace scope).
func (h *IdentityHandler) Create(c *echo.Context) error {
	return h.create(c)
}

// CreateGlobal handles POST /global/adapters/:id/identities.
func (h *IdentityHandler) CreateGlobal(c *echo.Context) error {
	return h.create(c)
}

func (h *IdentityHandler) create(c *echo.Context) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	var req request.CreateManualIdentityRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.Identity == "" {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "identity", Message: "is required"},
		)
	}

	identity, err := h.svc.CreateManual(c.Request().Context(), adapterID, req.Identity, req.DisplayName)
	if err != nil {
		return mapIdentityError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewAdapterIdentityResponse(identity))
}

// Delete handles DELETE .../adapters/:id/identities/:identity_id (workspace scope).
func (h *IdentityHandler) Delete(c *echo.Context) error {
	return h.deleteIdentity(c)
}

// DeleteGlobal handles DELETE /global/adapters/:id/identities/:identity_id.
func (h *IdentityHandler) DeleteGlobal(c *echo.Context) error {
	return h.deleteIdentity(c)
}

func (h *IdentityHandler) deleteIdentity(c *echo.Context) error {
	identityID, err := uuid.Parse(c.Param("identity_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid identity ID")
	}

	if err := h.svc.DeleteIdentity(c.Request().Context(), identityID); err != nil {
		return mapIdentityError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// SetDefault handles POST .../adapters/:id/identities/:identity_id/set-default (workspace scope).
func (h *IdentityHandler) SetDefault(c *echo.Context) error {
	return h.setDefault(c)
}

// SetDefaultGlobal handles POST /global/adapters/:id/identities/:identity_id/set-default.
func (h *IdentityHandler) SetDefaultGlobal(c *echo.Context) error {
	return h.setDefault(c)
}

func (h *IdentityHandler) setDefault(c *echo.Context) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	identityID, err := uuid.Parse(c.Param("identity_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid identity ID")
	}

	if err := h.svc.SetDefault(c.Request().Context(), adapterID, identityID); err != nil {
		return mapIdentityError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// mapIdentityError maps identity-specific domain errors to HTTP responses.
func mapIdentityError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrIdentityNotFound):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrIdentityNotInDomain):
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, domain.ErrNoDefaultIdentity):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrValidation):
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	default:
		return mapStoreError(c, err)
	}
}

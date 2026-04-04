package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// IdentityHandler handles adapter identity management endpoints.
type IdentityHandler struct {
	svc       *service.IdentityService
	store     port.AdapterIdentityStore
	tsStore   port.TenantStore
	wsStore   port.WorkspaceStore
	accessSvc *service.AdapterAccessService
	auditStore port.AuditLogStore
}

// NewIdentityHandler creates a new IdentityHandler.
func NewIdentityHandler(svc *service.IdentityService, store port.AdapterIdentityStore, ts port.TenantStore, ws port.WorkspaceStore) *IdentityHandler {
	return &IdentityHandler{svc: svc, store: store, tsStore: ts, wsStore: ws}
}

// SetAdapterAccessService wires workspace adapter/identity sharing behavior without widening constructor churn.
func (h *IdentityHandler) SetAdapterAccessService(accessSvc *service.AdapterAccessService) {
	h.accessSvc = accessSvc
}

// SetAuditStore wires audit logging for shared SES identity access mutations.
func (h *IdentityHandler) SetAuditStore(auditStore port.AuditLogStore) {
	h.auditStore = auditStore
}

// List handles GET .../adapters/:id/identities (workspace scope).
func (h *IdentityHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.list(c, ws)
}

// ListGlobal handles GET /global/adapters/:id/identities.
func (h *IdentityHandler) ListGlobal(c *echo.Context) error {
	return h.list(c, nil)
}

func (h *IdentityHandler) list(c *echo.Context, workspace *domain.Workspace) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	var identities []*domain.AdapterIdentity
	if workspace != nil && h.accessSvc != nil {
		identities, err = h.accessSvc.ListIdentitiesForWorkspace(c.Request().Context(), workspace, adapterID)
	} else {
		identities, err = h.svc.ListIdentities(c.Request().Context(), adapterID)
	}
	if err != nil {
		return mapIdentityAccessError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterIdentityListResponse(identities))
}

// Sync handles POST .../adapters/:id/identities/sync (workspace scope).
func (h *IdentityHandler) Sync(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.sync(c, ws)
}

// SyncGlobal handles POST /global/adapters/:id/identities/sync.
func (h *IdentityHandler) SyncGlobal(c *echo.Context) error {
	return h.sync(c, nil)
}

func (h *IdentityHandler) sync(c *echo.Context, workspace *domain.Workspace) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}
	if err := h.ensureEditableAdapter(c.Request().Context(), workspace, adapterID); err != nil {
		return mapIdentityAccessError(c, err)
	}

	identities, err := h.svc.SyncIdentities(c.Request().Context(), adapterID)
	if err != nil {
		return mapIdentityError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterIdentityListResponse(identities))
}

// Create handles POST .../adapters/:id/identities (workspace scope).
func (h *IdentityHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.create(c, ws)
}

// CreateGlobal handles POST /global/adapters/:id/identities.
func (h *IdentityHandler) CreateGlobal(c *echo.Context) error {
	return h.create(c, nil)
}

func (h *IdentityHandler) create(c *echo.Context, workspace *domain.Workspace) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}
	if err := h.ensureEditableAdapter(c.Request().Context(), workspace, adapterID); err != nil {
		return mapIdentityAccessError(c, err)
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
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.deleteIdentity(c, ws)
}

// DeleteGlobal handles DELETE /global/adapters/:id/identities/:identity_id.
func (h *IdentityHandler) DeleteGlobal(c *echo.Context) error {
	return h.deleteIdentity(c, nil)
}

func (h *IdentityHandler) deleteIdentity(c *echo.Context, workspace *domain.Workspace) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}
	if err := h.ensureEditableAdapter(c.Request().Context(), workspace, adapterID); err != nil {
		return mapIdentityAccessError(c, err)
	}

	identityID, err := uuid.Parse(c.Param("identity_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid identity ID")
	}
	identity, err := h.svc.GetByID(c.Request().Context(), identityID)
	if err != nil {
		return mapIdentityError(c, err)
	}
	if identity.AdapterID != adapterID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	if err := h.svc.DeleteIdentity(c.Request().Context(), identityID); err != nil {
		return mapIdentityError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// SetDefault handles POST .../adapters/:id/identities/:identity_id/set-default (workspace scope).
func (h *IdentityHandler) SetDefault(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.setDefault(c, ws)
}

// SetDefaultGlobal handles POST /global/adapters/:id/identities/:identity_id/set-default.
func (h *IdentityHandler) SetDefaultGlobal(c *echo.Context) error {
	return h.setDefault(c, nil)
}

func (h *IdentityHandler) setDefault(c *echo.Context, workspace *domain.Workspace) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}
	if err := h.ensureEditableAdapter(c.Request().Context(), workspace, adapterID); err != nil {
		return mapIdentityAccessError(c, err)
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

// GetWorkspaceAccess handles GET .../identities/:identity_id/workspace-access (workspace scope).
// Only tenant _system may manage SES identity sharing.
func (h *IdentityHandler) GetWorkspaceAccess(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	if h.accessSvc == nil {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}
	identityID, err := uuid.Parse(c.Param("identity_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid identity ID")
	}

	grants, err := h.accessSvc.ListIdentityWorkspaceAccess(c.Request().Context(), ws, adapterID, identityID)
	if err != nil {
		return mapIdentityAccessError(c, err)
	}
	return c.JSON(http.StatusOK, response.NewWorkspaceAccessListResponse(grants))
}

// UpdateWorkspaceAccess handles PUT .../identities/:identity_id/workspace-access (workspace scope).
// Only tenant _system may manage SES identity sharing.
func (h *IdentityHandler) UpdateWorkspaceAccess(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	if h.accessSvc == nil {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}
	identityID, err := uuid.Parse(c.Param("identity_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid identity ID")
	}

	var req request.UpdateWorkspaceAccessRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}
	workspaceIDs := make([]uuid.UUID, 0, len(req.WorkspaceIDs))
	for _, raw := range req.WorkspaceIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "workspace_ids", Message: "must contain valid UUIDs"},
			)
		}
		workspaceIDs = append(workspaceIDs, id)
	}

	before, err := h.accessSvc.ListIdentityWorkspaceAccess(c.Request().Context(), ws, adapterID, identityID)
	if err != nil {
		return mapIdentityAccessError(c, err)
	}

	if err := h.accessSvc.ReplaceIdentityWorkspaceAccess(c.Request().Context(), ws, adapterID, identityID, workspaceIDs); err != nil {
		return mapIdentityAccessError(c, err)
	}
	grants, err := h.accessSvc.ListIdentityWorkspaceAccess(c.Request().Context(), ws, adapterID, identityID)
	if err != nil {
		return mapIdentityAccessError(c, err)
	}

	identity, err := h.store.GetByID(c.Request().Context(), identityID)
	if err == nil {
		appendAuditLog(c, h.auditStore, newAuditEntry(
			"adapter_identity",
			identityID,
			ws.TenantID,
			ws.ID,
			newWorkspaceAccessAuditChanges(before, grants),
			map[string]any{
				"adapter_id":    adapterID.String(),
				"identity":      identity.Identity,
				"identity_type": identity.IdentityType,
			},
		))
	}
	return c.JSON(http.StatusOK, response.NewWorkspaceAccessListResponse(grants))
}

func (h *IdentityHandler) ensureEditableAdapter(ctx context.Context, workspace *domain.Workspace, adapterID uuid.UUID) error {
	if workspace == nil || h.accessSvc == nil {
		return nil
	}
	access, err := h.accessSvc.GetAdapterAccess(ctx, workspace, adapterID)
	if err != nil {
		return err
	}
	if !access.Editable {
		return domain.ErrSharedResourceReadOnly
	}
	return nil
}

func mapIdentityAccessError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrSharedResourceReadOnly):
		return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "shared resource is read-only")
	case errors.Is(err, domain.ErrSharedGrantInUse):
		return response.WriteError(c, http.StatusConflict, "CONFLICT", "shared grant is in use")
	case errors.Is(err, domain.ErrAdapterAccessDenied), errors.Is(err, domain.ErrSenderIdentityAccessDenied):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	default:
		return mapIdentityError(c, err)
	}
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

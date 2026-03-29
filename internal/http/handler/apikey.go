package handler

import (
	"net/http"
	"unicode"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// APIKeyHandler handles CRUD operations for API keys.
type APIKeyHandler struct {
	svc     *service.APIKeyService
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(svc *service.APIKeyService, ts port.TenantStore, ws port.WorkspaceStore) *APIKeyHandler {
	return &APIKeyHandler{svc: svc, tsStore: ts, wsStore: ws}
}

// Create handles POST /tenants/:tenant_code/workspaces/:workspace_code/api-keys.
// Returns the full key ONLY on creation (never again).
func (h *APIKeyHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.CreateAPIKeyRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	// Validate name: required, max 100 chars, no control characters.
	var fieldErrors []response.FieldError
	if req.Name == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	} else {
		if len(req.Name) > 100 {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "must be at most 100 characters"})
		}
		for _, r := range req.Name {
			if unicode.IsControl(r) {
				fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "must not contain control characters"})
				break
			}
		}
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	member, ok := c.Get("member").(*domain.Member)
	if !ok || member == nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	fullKey, key, err := h.svc.Generate(c.Request().Context(), ws.ID, req.Name, member.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewAPIKeyCreatedResponse(key, fullKey))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/api-keys.
// Paginated, NEVER shows key hash or full key.
func (h *APIKeyHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	opts := pagination.ParseListOptions(c)

	page, err := h.svc.ListByWorkspace(c.Request().Context(), ws.ID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAPIKeyListResponse(page))
}

// Revoke handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/api-keys/:id.
func (h *APIKeyHandler) Revoke(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid API key ID")
	}

	if err := h.svc.Revoke(c.Request().Context(), ws.ID, keyID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

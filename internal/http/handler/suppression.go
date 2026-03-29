package handler

import (
	"net/http"
	"net/mail"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// SuppressionHandler handles suppression list management (OIDC management auth).
type SuppressionHandler struct {
	store   port.SuppressionStore
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewSuppressionHandler creates a new SuppressionHandler.
func NewSuppressionHandler(ss port.SuppressionStore, ts port.TenantStore, ws port.WorkspaceStore) *SuppressionHandler {
	return &SuppressionHandler{store: ss, tsStore: ts, wsStore: ws}
}

// Add handles POST /tenants/:tenant_code/workspaces/:workspace_code/suppression.
func (h *SuppressionHandler) Add(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.AddSuppressionRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.Email == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "email", Message: "is required"})
	} else if _, err := mail.ParseAddress(req.Email); err != nil {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "email", Message: "must be a valid email address"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	reason := domain.SuppressionManual
	if req.Reason != "" {
		r := domain.SuppressionReason(req.Reason)
		switch r {
		case domain.SuppressionManual, domain.SuppressionHardBounce, domain.SuppressionComplaint:
			reason = r
		default:
			return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid suppression reason")
		}
	}

	entry := &domain.SuppressionWorkspace{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: ws.ID,
		Email:       req.Email,
		Reason:      reason,
		Notes:       req.Notes,
		CreatedAt:   time.Now().UTC(),
	}

	if err := h.store.AddWorkspace(c.Request().Context(), entry); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewSuppressionWorkspaceResponse(entry))
}

// Check handles GET /tenants/:tenant_code/workspaces/:workspace_code/suppression/:email.
func (h *SuppressionHandler) Check(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	email := c.Param("email")
	if email == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "email is required")
	}

	suppressed, reason, err := h.store.IsSuppressed(c.Request().Context(), ws.ID, email)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.SuppressionCheckResponse{
		Email:      email,
		Suppressed: suppressed,
		Reason:     reason,
	})
}

// Remove handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/suppression/:email.
//
// IMPORTANT: SuppressionStore only has RemoveGlobal — no workspace-scoped remove.
// To prevent privilege escalation (workspace admin removing global suppressions),
// this endpoint requires superadmin role. When port.SuppressionStore gains a
// RemoveWorkspace method, this handler should be updated to use it for
// workspace-level callers.
func (h *SuppressionHandler) Remove(c *echo.Context) error {
	_, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	email := c.Param("email")
	if email == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "email is required")
	}

	member, ok := c.Get("member").(*domain.Member)
	if !ok || member == nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	// Require superadmin role since RemoveGlobal affects all workspaces.
	// Without a workspace-scoped RemoveWorkspace method, allowing any workspace
	// admin to remove global suppressions would be privilege escalation.
	roles, _ := c.Get("roles").([]*domain.MemberRole)
	isSuperadmin := false
	for _, r := range roles {
		if r.Role == domain.RoleSuperadmin {
			isSuperadmin = true
			break
		}
	}
	if !isSuperadmin {
		return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "superadmin role required for global suppression removal")
	}

	reason := c.QueryParam("reason")
	if reason == "" {
		reason = "manual removal"
	}

	if err := h.store.RemoveGlobal(c.Request().Context(), email, member.ID, reason); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

package handler

import (
	"net/http"
	"net/mail"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// MemberHandler handles CRUD operations for members and their roles.
type MemberHandler struct {
	store port.MemberStore
}

// NewMemberHandler creates a new MemberHandler.
func NewMemberHandler(ms port.MemberStore) *MemberHandler {
	return &MemberHandler{store: ms}
}

// List handles GET /api/v1/manage/members (paginated, with roles).
func (h *MemberHandler) List(c *echo.Context) error {
	opts := pagination.ParseListOptions(c)
	ctx := c.Request().Context()

	members, nextCursor, err := h.store.ListAll(ctx, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	// Batch fetch all roles in a single query instead of N+1.
	memberIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		memberIDs[i] = m.ID
	}

	rolesByMember, err := h.store.GetRolesByMembers(ctx, memberIDs)
	if err != nil {
		return mapStoreError(c, err)
	}

	items := make([]response.MemberWithRolesResponse, len(members))
	for i, m := range members {
		roles := rolesByMember[m.ID]
		roleResponses := make([]response.MemberRoleResponse, len(roles))
		for j, r := range roles {
			roleResponses[j] = response.NewMemberRoleResponse(r)
		}
		items[i] = response.MemberWithRolesResponse{
			MemberResponse: response.NewMemberResponse(m),
			Roles:          roleResponses,
		}
	}

	return c.JSON(http.StatusOK, response.MemberListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Create handles POST /api/v1/manage/members.
func (h *MemberHandler) Create(c *echo.Context) error {
	var req request.CreateMemberRequest
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

	now := time.Now().UTC()
	member := &domain.Member{
		ID:          uuid.Must(uuid.NewV7()),
		Email:       req.Email,
		DisplayName: req.DisplayName,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.Create(c.Request().Context(), member); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewMemberResponse(member))
}

// Get handles GET /api/v1/manage/members/:member_id.
func (h *MemberHandler) Get(c *echo.Context) error {
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member ID")
	}

	ctx := c.Request().Context()
	member, err := h.store.GetByID(ctx, memberID)
	if err != nil {
		return mapStoreError(c, err)
	}

	roles, err := h.store.GetRoles(ctx, memberID)
	if err != nil {
		return mapStoreError(c, err)
	}

	roleResponses := make([]response.MemberRoleResponse, len(roles))
	for i, r := range roles {
		roleResponses[i] = response.NewMemberRoleResponse(r)
	}

	return c.JSON(http.StatusOK, response.MemberWithRolesResponse{
		MemberResponse: response.NewMemberResponse(member),
		Roles:          roleResponses,
	})
}

// Me handles GET /api/v1/members/me.
func (h *MemberHandler) Me(c *echo.Context) error {
	member, _ := c.Get(middleware.ContextKeyMember).(*domain.Member)
	if member == nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing member context")
	}

	roles, _ := c.Get(middleware.ContextKeyRoles).([]*domain.MemberRole)
	roleResponses := make([]response.MemberRoleResponse, len(roles))
	for i, r := range roles {
		roleResponses[i] = response.NewMemberRoleResponse(r)
	}

	return c.JSON(http.StatusOK, response.MemberWithRolesResponse{
		MemberResponse: response.NewMemberResponse(member),
		Roles:          roleResponses,
	})
}

// AddRole handles POST /api/v1/manage/members/:member_id/roles.
func (h *MemberHandler) AddRole(c *echo.Context) error {
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member ID")
	}

	ctx := c.Request().Context()
	if _, err := h.store.GetByID(ctx, memberID); err != nil {
		return mapStoreError(c, err)
	}

	var req request.AddRoleRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	role := domain.Role(req.Role)
	if !isValidRole(role) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "role", Message: "must be one of: superadmin, tenant_admin, workspace_admin, workspace_editor, workspace_viewer"})
	}
	scopeType := domain.ScopeType(req.ScopeType)
	if !isValidScopeType(scopeType) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "scope_type", Message: "must be one of: global, tenant, workspace"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	mr := &domain.MemberRole{
		ID:        uuid.Must(uuid.NewV7()),
		MemberID:  memberID,
		Role:      role,
		ScopeType: scopeType,
		CreatedAt: time.Now().UTC(),
	}

	if req.TenantID != nil {
		tid, err := uuid.Parse(*req.TenantID)
		if err != nil {
			return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid tenant_id")
		}
		mr.TenantID = &tid
	}
	if req.WorkspaceID != nil {
		wid, err := uuid.Parse(*req.WorkspaceID)
		if err != nil {
			return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid workspace_id")
		}
		mr.WorkspaceID = &wid
	}

	if err := h.store.AddRole(ctx, mr); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewMemberRoleResponse(mr))
}

// RemoveRole handles DELETE /api/v1/manage/members/:member_id/roles/:role_id.
func (h *MemberHandler) RemoveRole(c *echo.Context) error {
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member ID")
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid role ID")
	}

	ctx := c.Request().Context()
	// Verify member exists.
	if _, err := h.store.GetByID(ctx, memberID); err != nil {
		return mapStoreError(c, err)
	}

	if err := h.store.RemoveRole(ctx, roleID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func isValidRole(r domain.Role) bool {
	switch r {
	case domain.RoleSuperadmin, domain.RoleTenantAdmin, domain.RoleWorkspaceAdmin,
		domain.RoleWorkspaceEditor, domain.RoleWorkspaceViewer:
		return true
	}
	return false
}

func isValidScopeType(s domain.ScopeType) bool {
	switch s {
	case domain.ScopeGlobal, domain.ScopeTenant, domain.ScopeWorkspace:
		return true
	}
	return false
}

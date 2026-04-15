package handler

import (
	"context"
	"errors"
	"log/slog"
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
	"github.com/rendis/senda/pkg/apperr"
)

type memberScope struct {
	scopeType   domain.ScopeType
	tenantID    *uuid.UUID
	workspaceID *uuid.UUID
}

func (s *memberScope) scopeID() *uuid.UUID {
	if s == nil {
		return nil
	}
	switch s.scopeType {
	case domain.ScopeTenant:
		return s.tenantID
	case domain.ScopeWorkspace:
		return s.workspaceID
	default:
		return nil
	}
}

func (s *memberScope) rolesForResponse(roles []*domain.MemberRole) []*domain.MemberRole {
	if s == nil {
		return roles
	}

	filtered := make([]*domain.MemberRole, 0, len(roles))
	for _, role := range roles {
		if s.matches(role) {
			filtered = append(filtered, role)
		}
	}
	return filtered
}

func (s *memberScope) matches(role *domain.MemberRole) bool {
	if s == nil || role == nil {
		return true
	}

	if role.ScopeType != s.scopeType {
		return false
	}

	switch s.scopeType {
	case domain.ScopeTenant:
		return role.TenantID != nil && s.tenantID != nil && *role.TenantID == *s.tenantID
	case domain.ScopeWorkspace:
		return role.WorkspaceID != nil && s.workspaceID != nil && *role.WorkspaceID == *s.workspaceID
	default:
		return true
	}
}

func (s *memberScope) allowedRole(role domain.Role) bool {
	if s == nil {
		return isValidRole(role)
	}

	switch s.scopeType {
	case domain.ScopeTenant:
		return role == domain.RoleTenantAdmin
	case domain.ScopeWorkspace:
		return role == domain.RoleWorkspaceAdmin || role == domain.RoleWorkspaceEditor || role == domain.RoleWorkspaceViewer
	default:
		return isValidRole(role)
	}
}

func (s *memberScope) allowedScopeType(scopeType domain.ScopeType) bool {
	if s == nil {
		return isValidScopeType(scopeType)
	}
	return scopeType == s.scopeType
}

// MemberHandler handles CRUD operations for members and their roles.
type MemberHandler struct {
	store       port.MemberStore
	tenantStore port.TenantStore
	wsStore     port.WorkspaceStore
}

// NewMemberHandler creates a new MemberHandler.
func NewMemberHandler(ms port.MemberStore, ts port.TenantStore, ws port.WorkspaceStore) *MemberHandler {
	return &MemberHandler{store: ms, tenantStore: ts, wsStore: ws}
}

// List handles GET /api/v1/manage/members (paginated, with roles).
func (h *MemberHandler) List(c *echo.Context) error {
	return h.list(c, nil)
}

// ListTenant handles GET /api/v1/manage/tenants/:tenant_code/members.
func (h *MemberHandler) ListTenant(c *echo.Context) error {
	scope, err := h.resolveTenantScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.list(c, scope)
}

// ListWorkspace handles GET /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members.
func (h *MemberHandler) ListWorkspace(c *echo.Context) error {
	scope, err := h.resolveWorkspaceScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.list(c, scope)
}

func (h *MemberHandler) list(c *echo.Context, scope *memberScope) error {
	opts := pagination.ParseListOptions(c)
	ctx := c.Request().Context()

	var (
		members     []*domain.Member
		nextCursor  string
		err         error
		filterRoles bool
	)

	if scope == nil {
		members, nextCursor, err = h.store.ListAll(ctx, opts)
	} else {
		members, nextCursor, err = h.store.ListInScope(ctx, scope.scopeType, scope.scopeID(), opts)
		filterRoles = true
	}
	if err != nil {
		return mapStoreError(c, err)
	}

	memberIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		memberIDs[i] = m.ID
	}

	rolesByMember, err := h.store.GetRolesByMembers(ctx, memberIDs)
	if err != nil {
		return mapStoreError(c, err)
	}

	items := make([]response.MemberWithRolesResponse, 0, len(members))
	for _, m := range members {
		roles := rolesByMember[m.ID]
		if filterRoles {
			roles = scope.rolesForResponse(roles)
		}
		if scope != nil && len(roles) == 0 {
			continue
		}

		roleResponses := make([]response.MemberRoleResponse, len(roles))
		for i, r := range roles {
			roleResponses[i] = response.NewMemberRoleResponse(r)
		}
		items = append(items, response.MemberWithRolesResponse{
			MemberResponse: response.NewMemberResponse(m),
			Roles:          roleResponses,
		})
	}

	return c.JSON(http.StatusOK, response.MemberListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Create handles POST /api/v1/manage/members.
func (h *MemberHandler) Create(c *echo.Context) error {
	scope, err := h.createScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}

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

	var requestedRole domain.Role
	var hasRequestedRole bool
	if req.Role != nil && *req.Role != "" {
		requestedRole = domain.Role(*req.Role)
		hasRequestedRole = true
	}

	if scope != nil {
		if !hasRequestedRole {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "role", Message: "is required for scoped member creation"})
		} else if !scope.allowedRole(requestedRole) {
			fieldErrors = append(fieldErrors, response.FieldError{
				Field:   "role",
				Message: "role is not allowed for this scope",
			})
		}
	} else if hasRequestedRole {
		fieldErrors = append(fieldErrors, response.FieldError{
			Field:   "role",
			Message: "role is only allowed on scoped member creation",
		})
	}

	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	ctx := c.Request().Context()

	member, err := h.store.GetByEmail(ctx, req.Email)
	if err != nil {
		if !apperr.IsNotFound(err) {
			return mapStoreError(c, err)
		}
		member = nil
	}

	if member == nil {
		now := time.Now().UTC()
		member = &domain.Member{
			ID:          uuid.Must(uuid.NewV7()),
			Email:       req.Email,
			DisplayName: req.DisplayName,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := h.store.Create(ctx, member); err != nil {
			return mapStoreError(c, err)
		}
	}

	roles := make([]*domain.MemberRole, 0, 1)
	if scope != nil && hasRequestedRole {
		memberRole := &domain.MemberRole{
			ID:        uuid.Must(uuid.NewV7()),
			MemberID:  member.ID,
			Role:      requestedRole,
			ScopeType: scope.scopeType,
			CreatedAt: time.Now().UTC(),
		}
		if scope.tenantID != nil {
			tenantID := *scope.tenantID
			memberRole.TenantID = &tenantID
		}
		if scope.workspaceID != nil {
			workspaceID := *scope.workspaceID
			memberRole.WorkspaceID = &workspaceID
		}
		if err := h.store.AddRole(ctx, memberRole); err != nil {
			return mapStoreError(c, err)
		}
		roles = append(roles, memberRole)
	}

	roleResponses := make([]response.MemberRoleResponse, len(roles))
	for i, r := range roles {
		roleResponses[i] = response.NewMemberRoleResponse(r)
	}

	return c.JSON(http.StatusCreated, response.MemberWithRolesResponse{
		MemberResponse: response.NewMemberResponse(member),
		Roles:          roleResponses,
	})
}

// Get handles GET /api/v1/manage/members/:member_id.
func (h *MemberHandler) Get(c *echo.Context) error {
	return h.get(c, nil)
}

// GetTenant handles GET /api/v1/manage/tenants/:tenant_code/members/:member_id.
func (h *MemberHandler) GetTenant(c *echo.Context) error {
	scope, err := h.resolveTenantScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.get(c, scope)
}

// GetWorkspace handles GET /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id.
func (h *MemberHandler) GetWorkspace(c *echo.Context) error {
	scope, err := h.resolveWorkspaceScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.get(c, scope)
}

func (h *MemberHandler) get(c *echo.Context, scope *memberScope) error {
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member ID")
	}

	ctx := c.Request().Context()
	member, err := h.store.GetByID(ctx, memberID)
	if err != nil {
		return mapStoreError(c, err)
	}

	roles, err := h.scopedRoles(ctx, memberID, scope)
	if err != nil {
		return mapStoreError(c, err)
	}
	if scope != nil && len(roles) == 0 {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
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

	h.enrichRoleCodes(c.Request().Context(), roleResponses, roles)

	return c.JSON(http.StatusOK, response.MemberWithRolesResponse{
		MemberResponse: response.NewMemberResponse(member),
		Roles:          roleResponses,
	})
}

// enrichRoleCodes resolves tenant/workspace IDs to their codes so the frontend
// can match roles against URL path segments without extra API calls.
func (h *MemberHandler) enrichRoleCodes(ctx context.Context, responses []response.MemberRoleResponse, roles []*domain.MemberRole) { //nolint:gocognit // resolves tenant/workspace codes for role responses
	tenantCodes := make(map[uuid.UUID]string)
	workspaceCodes := make(map[uuid.UUID]string)

	for _, r := range roles {
		if r.TenantID != nil {
			if _, ok := tenantCodes[*r.TenantID]; !ok {
				if t, err := h.tenantStore.GetByID(ctx, *r.TenantID); err == nil {
					tenantCodes[*r.TenantID] = t.Code
				} else {
					slog.Warn("enrichRoleCodes: failed to resolve tenant", slog.String("tenant_id", r.TenantID.String()), slog.String("error", err.Error()))
				}
			}
		}
		if r.WorkspaceID != nil {
			if _, ok := workspaceCodes[*r.WorkspaceID]; !ok {
				if ws, err := h.wsStore.GetByID(ctx, *r.WorkspaceID); err == nil {
					workspaceCodes[*r.WorkspaceID] = ws.Code
				} else {
					slog.Warn("enrichRoleCodes: failed to resolve workspace", slog.String("workspace_id", r.WorkspaceID.String()), slog.String("error", err.Error()))
				}
			}
		}
	}

	for i, r := range roles {
		if r.TenantID != nil {
			if code, ok := tenantCodes[*r.TenantID]; ok {
				responses[i].TenantCode = &code
			}
		}
		if r.WorkspaceID != nil {
			if code, ok := workspaceCodes[*r.WorkspaceID]; ok {
				responses[i].WorkspaceCode = &code
			}
		}
	}
}

// AddRole handles POST /api/v1/manage/members/:member_id/roles.
func (h *MemberHandler) AddRole(c *echo.Context) error {
	return h.addRole(c, nil)
}

// ReplaceRole handles PUT /api/v1/manage/members/:member_id/role.
func (h *MemberHandler) ReplaceRole(c *echo.Context) error {
	return h.replaceRole(c, nil)
}

// AddRoleTenant handles POST /api/v1/manage/tenants/:tenant_code/members/:member_id/roles.
func (h *MemberHandler) AddRoleTenant(c *echo.Context) error {
	scope, err := h.resolveTenantScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.addRole(c, scope)
}

// ReplaceRoleTenant handles PUT /api/v1/manage/tenants/:tenant_code/members/:member_id/role.
func (h *MemberHandler) ReplaceRoleTenant(c *echo.Context) error {
	scope, err := h.resolveTenantScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.replaceRole(c, scope)
}

// AddRoleWorkspace handles POST /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/roles.
func (h *MemberHandler) AddRoleWorkspace(c *echo.Context) error {
	scope, err := h.resolveWorkspaceScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.addRole(c, scope)
}

// ReplaceRoleWorkspace handles PUT /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/role.
func (h *MemberHandler) ReplaceRoleWorkspace(c *echo.Context) error {
	scope, err := h.resolveWorkspaceScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.replaceRole(c, scope)
}

func (h *MemberHandler) addRole(c *echo.Context, scope *memberScope) error {
	return h.mutateRole(c, scope, roleMutation{
		persist: func(ctx context.Context, role *domain.MemberRole) error {
			return h.store.AddRole(ctx, role)
		},
		status: http.StatusCreated,
	})
}

func (h *MemberHandler) replaceRole(c *echo.Context, scope *memberScope) error {
	return h.mutateRole(c, scope, roleMutation{
		persist: func(ctx context.Context, role *domain.MemberRole) error {
			return h.store.ReplaceRoleInScope(ctx, role)
		},
		status: http.StatusOK,
	})
}

type roleMutation struct {
	persist func(context.Context, *domain.MemberRole) error
	status  int
}

type roleAssignmentRequest struct {
	Role        string  `json:"role"`
	ScopeType   string  `json:"scope_type"`
	TenantID    *string `json:"tenant_id"`
	WorkspaceID *string `json:"workspace_id"`
}

type roleAssignmentParseError struct {
	message string
}

func (e roleAssignmentParseError) Error() string {
	return e.message
}

func (h *MemberHandler) mutateRole(c *echo.Context, scope *memberScope, mutation roleMutation) error { //nolint:gocognit,gocyclo,funlen // role validation with scope-dependent logic
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member ID")
	}

	ctx := c.Request().Context()
	if _, err := h.store.GetByID(ctx, memberID); err != nil {
		return mapStoreError(c, err)
	}

	var req roleAssignmentRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	mr, fieldErrors, err := buildMemberRole(memberID, scope, req)
	if err != nil {
		var parseErr roleAssignmentParseError
		if errors.As(err, &parseErr) {
			return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", parseErr.message)
		}
		return err
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	if err := mutation.persist(ctx, mr); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(mutation.status, response.NewMemberRoleResponse(mr))
}

func buildMemberRole(memberID uuid.UUID, scope *memberScope, req roleAssignmentRequest) (*domain.MemberRole, []response.FieldError, error) {
	role := domain.Role(req.Role)
	scopeType := domain.ScopeType(req.ScopeType)
	if scope != nil {
		scopeType = scope.scopeType
	}
	fieldErrors := validateRoleAssignment(scope, req, role, scopeType)

	mr := &domain.MemberRole{
		ID:        uuid.Must(uuid.NewV7()),
		MemberID:  memberID,
		Role:      role,
		ScopeType: scopeType,
		CreatedAt: time.Now().UTC(),
	}

	if scope != nil {
		if scope.tenantID != nil {
			tenantID := *scope.tenantID
			mr.TenantID = &tenantID
		}
		if scope.workspaceID != nil {
			workspaceID := *scope.workspaceID
			mr.WorkspaceID = &workspaceID
		}
		return mr, fieldErrors, nil
	}

	var err error
	mr.TenantID, err = parseOptionalUUID(req.TenantID)
	if err != nil {
		return nil, nil, roleAssignmentParseError{message: "invalid tenant_id"}
	}
	mr.WorkspaceID, err = parseOptionalUUID(req.WorkspaceID)
	if err != nil {
		return nil, nil, roleAssignmentParseError{message: "invalid workspace_id"}
	}

	return mr, fieldErrors, nil
}

func validateRoleAssignment(scope *memberScope, req roleAssignmentRequest, role domain.Role, scopeType domain.ScopeType) []response.FieldError {
	var fieldErrors []response.FieldError
	if !scope.allowedRole(role) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "role", Message: "role is not allowed for this scope"})
	}
	fieldErrors = append(fieldErrors, validateScopedRoleAssignment(scope, req)...)
	fieldErrors = append(fieldErrors, validateRoleScopeType(scope, scopeType)...)
	return fieldErrors
}

func validateScopedRoleAssignment(scope *memberScope, req roleAssignmentRequest) []response.FieldError {
	if scope == nil {
		return nil
	}

	var fieldErrors []response.FieldError
	if req.ScopeType != "" && domain.ScopeType(req.ScopeType) != scope.scopeType {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "scope_type", Message: "scope_type must match the route scope"})
	}
	fieldErrors = append(fieldErrors, validateScopedUUID(req.TenantID, scope.tenantID, "tenant_id", "tenant")...)
	fieldErrors = append(fieldErrors, validateScopedUUID(req.WorkspaceID, scope.workspaceID, "workspace_id", "workspace")...)
	return fieldErrors
}

func validateScopedUUID(raw *string, expected *uuid.UUID, field string, target string) []response.FieldError {
	if raw == nil {
		return nil
	}

	parsed, err := uuid.Parse(*raw)
	if err != nil {
		return []response.FieldError{{Field: field, Message: "must be a valid UUID"}}
	}
	if expected != nil && parsed != *expected {
		return []response.FieldError{{Field: field, Message: "must match the route " + target}}
	}
	return nil
}

func validateRoleScopeType(scope *memberScope, scopeType domain.ScopeType) []response.FieldError {
	var fieldErrors []response.FieldError
	if !scope.allowedScopeType(scopeType) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "scope_type", Message: "scope_type is not allowed for this route"})
	}

	if scope == nil {
		if !isValidScopeType(scopeType) {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "scope_type", Message: "must be one of: global, tenant, workspace"})
		}
		return fieldErrors
	}

	if scopeType == domain.ScopeWorkspace && scope.workspaceID == nil {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "scope_type", Message: "workspace scope requires a workspace"})
	}
	if scopeType == domain.ScopeTenant && scope.tenantID == nil {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "scope_type", Message: "tenant scope requires a tenant"})
	}
	return fieldErrors
}

// RemoveRole handles DELETE /api/v1/manage/members/:member_id/roles/:role_id.
func (h *MemberHandler) RemoveRole(c *echo.Context) error {
	return h.removeRole(c, nil)
}

// RemoveRoleTenant handles DELETE /api/v1/manage/tenants/:tenant_code/members/:member_id/roles/:role_id.
func (h *MemberHandler) RemoveRoleTenant(c *echo.Context) error {
	scope, err := h.resolveTenantScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.removeRole(c, scope)
}

// RemoveRoleWorkspace handles DELETE /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/roles/:role_id.
func (h *MemberHandler) RemoveRoleWorkspace(c *echo.Context) error {
	scope, err := h.resolveWorkspaceScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.removeRole(c, scope)
}

// RemoveAccess handles DELETE /api/v1/manage/members/:member_id/access.
func (h *MemberHandler) RemoveAccess(c *echo.Context) error {
	return h.removeAccess(c, nil)
}

// RemoveAccessTenant handles DELETE /api/v1/manage/tenants/:tenant_code/members/:member_id/access.
func (h *MemberHandler) RemoveAccessTenant(c *echo.Context) error {
	scope, err := h.resolveTenantScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.removeAccess(c, scope)
}

// RemoveAccessWorkspace handles DELETE /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/access.
func (h *MemberHandler) RemoveAccessWorkspace(c *echo.Context) error {
	scope, err := h.resolveWorkspaceScope(c)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.removeAccess(c, scope)
}

func (h *MemberHandler) removeRole(c *echo.Context, scope *memberScope) error {
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member ID")
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid role ID")
	}

	ctx := c.Request().Context()
	if _, err := h.store.GetByID(ctx, memberID); err != nil {
		return mapStoreError(c, err)
	}

	if scope != nil {
		roles, err := h.scopedRoles(ctx, memberID, scope)
		if err != nil {
			return mapStoreError(c, err)
		}
		found := false
		for _, role := range roles {
			if role.ID == roleID {
				found = true
				break
			}
		}
		if !found {
			return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
		}
	}

	if err := h.store.RemoveRole(ctx, roleID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MemberHandler) removeAccess(c *echo.Context, scope *memberScope) error {
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member ID")
	}

	ctx := c.Request().Context()
	if _, err := h.store.GetByID(ctx, memberID); err != nil {
		return mapStoreError(c, err)
	}

	var scopeType domain.ScopeType
	var scopeID *uuid.UUID
	if scope == nil {
		scopeType = domain.ScopeGlobal
	} else {
		scopeType = scope.scopeType
		scopeID = scope.scopeID()
	}

	if scope == nil && isGlobalSelfRevoke(c, memberID) {
		return response.WriteError(c, http.StatusConflict, "CONFLICT", "cannot revoke your own global superadmin access")
	}

	if _, err := h.store.RevokeAccessInScope(ctx, memberID, scopeType, scopeID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func isGlobalSelfRevoke(c *echo.Context, targetMemberID uuid.UUID) bool {
	actor, _ := c.Get(middleware.ContextKeyMember).(*domain.Member)
	if actor == nil || actor.ID != targetMemberID {
		return false
	}

	roles, _ := c.Get(middleware.ContextKeyRoles).([]*domain.MemberRole)
	for _, role := range roles {
		if role != nil && role.Role == domain.RoleSuperadmin && role.ScopeType == domain.ScopeGlobal {
			return true
		}
	}

	return false
}

func (h *MemberHandler) scopedRoles(ctx context.Context, memberID uuid.UUID, scope *memberScope) ([]*domain.MemberRole, error) {
	if scope == nil {
		return h.store.GetRoles(ctx, memberID)
	}
	return h.store.GetRolesInScope(ctx, memberID, scope.scopeType, scope.scopeID())
}

func (h *MemberHandler) resolveTenantScope(c *echo.Context) (*memberScope, error) {
	if h.tenantStore == nil {
		return nil, response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "tenant store not configured")
	}
	tenant, err := resolveTenant(c, h.tenantStore)
	if err != nil {
		return nil, err
	}
	tenantID := tenant.ID
	return &memberScope{scopeType: domain.ScopeTenant, tenantID: &tenantID}, nil
}

func (h *MemberHandler) resolveWorkspaceScope(c *echo.Context) (*memberScope, error) {
	if h.tenantStore == nil || h.wsStore == nil {
		return nil, response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "workspace scope is not configured")
	}
	ws, err := resolveWorkspace(c, h.tenantStore, h.wsStore)
	if err != nil {
		return nil, err
	}
	tenantID := ws.TenantID
	return &memberScope{scopeType: domain.ScopeWorkspace, tenantID: &tenantID, workspaceID: &ws.ID}, nil
}

func (h *MemberHandler) createScope(c *echo.Context) (*memberScope, error) {
	if c.Param("workspace_code") != "" {
		return h.resolveWorkspaceScope(c)
	}
	if c.Param("tenant_code") != "" {
		return h.resolveTenantScope(c)
	}
	return nil, nil
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

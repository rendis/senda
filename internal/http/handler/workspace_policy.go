package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// WorkspacePolicyResponse exposes tenant-local workspace policies stored on the _system workspace.
type WorkspacePolicyResponse struct {
	AllowWorkspaceLocalTemplates         bool `json:"allow_workspace_local_templates"`
	AllowWorkspaceInheritedTemplateForks bool `json:"allow_workspace_inherited_template_forks"`
	AllowWorkspaceLocalInjectors         bool `json:"allow_workspace_local_injectors"`
}

type WorkspacePolicyHandler struct {
	tenantStore port.TenantStore
	wsStore     port.WorkspaceStore
}

func NewWorkspacePolicyHandler(ts port.TenantStore, ws port.WorkspaceStore) *WorkspacePolicyHandler {
	return &WorkspacePolicyHandler{tenantStore: ts, wsStore: ws}
}

func (h *WorkspacePolicyHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tenantStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	policyWorkspace := ws
	if !ws.IsSystem {
		policyWorkspace, err = resolveSystemWorkspace(c.Request().Context(), ws, h.wsStore)
		if err != nil {
			return mapStoreError(c, err)
		}
	}

	return c.JSON(http.StatusOK, newWorkspacePolicyResponse(policyWorkspace))
}

func (h *WorkspacePolicyHandler) Update(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tenantStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	if !ws.IsSystem {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	var req request.UpdateWorkspacePolicyRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.AllowWorkspaceLocalTemplates != nil {
		ws.AllowWorkspaceLocalTemplates = *req.AllowWorkspaceLocalTemplates
	}
	if req.AllowWorkspaceInheritedTemplateForks != nil {
		ws.AllowWorkspaceInheritedTemplateForks = *req.AllowWorkspaceInheritedTemplateForks
	}
	if req.AllowWorkspaceLocalInjectors != nil {
		ws.AllowWorkspaceLocalInjectors = *req.AllowWorkspaceLocalInjectors
	}
	ws.WorkspacePoliciesInitialized = true
	ws.UpdatedAt = time.Now().UTC()

	if err := h.wsStore.Update(c.Request().Context(), ws); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, newWorkspacePolicyResponse(ws))
}

func newWorkspacePolicyResponse(ws *domain.Workspace) WorkspacePolicyResponse {
	return WorkspacePolicyResponse(effectiveWorkspacePolicies(ws))
}

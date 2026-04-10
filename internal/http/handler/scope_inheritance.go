package handler

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
)

type workspacePolicies struct {
	AllowWorkspaceLocalTemplates         bool
	AllowWorkspaceInheritedTemplateForks bool
	AllowWorkspaceLocalInjectors         bool
}

func defaultWorkspacePolicies() workspacePolicies {
	return workspacePolicies{
		AllowWorkspaceLocalTemplates:         true,
		AllowWorkspaceInheritedTemplateForks: true,
		AllowWorkspaceLocalInjectors:         true,
	}
}

func effectiveWorkspacePolicies(systemWorkspace *domain.Workspace) workspacePolicies {
	if systemWorkspace == nil || !systemWorkspace.WorkspacePoliciesInitialized {
		return defaultWorkspacePolicies()
	}
	return workspacePolicies{
		AllowWorkspaceLocalTemplates:         systemWorkspace.AllowWorkspaceLocalTemplates,
		AllowWorkspaceInheritedTemplateForks: systemWorkspace.AllowWorkspaceInheritedTemplateForks,
		AllowWorkspaceLocalInjectors:         systemWorkspace.AllowWorkspaceLocalInjectors,
	}
}

func resolveSystemWorkspace(ctx context.Context, current *domain.Workspace, wsStore port.WorkspaceStore) (*domain.Workspace, error) {
	if current == nil {
		return nil, nil
	}
	if current.IsSystem {
		return current, nil
	}

	systemWorkspace, err := wsStore.GetSystemWorkspace(ctx, current.TenantID)
	if err != nil {
		if apperr.IsNotFound(err) || errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return systemWorkspace, nil
}

func workspaceResolutionChain(ctx context.Context, current *domain.Workspace, wsStore port.WorkspaceStore) ([]uuid.NullUUID, *domain.Workspace, error) {
	if current == nil {
		return []uuid.NullUUID{{Valid: false}}, nil, nil
	}

	systemWorkspace, err := resolveSystemWorkspace(ctx, current, wsStore)
	if err != nil {
		return nil, nil, err
	}

	chain := []uuid.NullUUID{uuidToNullUUID(&current.ID)}
	if systemWorkspace != nil && systemWorkspace.ID != current.ID {
		chain = append(chain, uuidToNullUUID(&systemWorkspace.ID))
	}
	return chain, systemWorkspace, nil
}

func resourceOwnerScope(resourceWorkspaceID *uuid.UUID, current *domain.Workspace, system *domain.Workspace) string {
	if resourceWorkspaceID == nil {
		return "global"
	}
	if current != nil && *resourceWorkspaceID == current.ID {
		if current.IsSystem {
			return "system"
		}
		return "local"
	}
	if system != nil && *resourceWorkspaceID == system.ID {
		return "system"
	}
	return "workspace"
}

func inheritedFromSystem(resourceWorkspaceID *uuid.UUID, current *domain.Workspace, system *domain.Workspace) bool {
	if current == nil || current.IsSystem || resourceWorkspaceID == nil || system == nil {
		return false
	}
	return *resourceWorkspaceID == system.ID
}

func annotateTemplateTypeScope(tt *domain.TemplateType, current *domain.Workspace, system *domain.Workspace) {
	if tt == nil {
		return
	}
	tt.OwnerScope = resourceOwnerScope(tt.WorkspaceID, current, system)
	tt.InheritedFromSystem = inheritedFromSystem(tt.WorkspaceID, current, system)
}

func annotateTemplateScope(tpl *domain.Template, current *domain.Workspace, system *domain.Workspace) {
	if tpl == nil {
		return
	}
	tpl.OwnerScope = resourceOwnerScope(tpl.WorkspaceID, current, system)
	tpl.InheritedFromSystem = inheritedFromSystem(tpl.WorkspaceID, current, system)
}

func annotateInjectorScope(def *domain.InjectorDefinition, current *domain.Workspace, system *domain.Workspace) {
	if def == nil {
		return
	}
	def.OwnerScope = resourceOwnerScope(def.WorkspaceID, current, system)
	def.InheritedFromSystem = inheritedFromSystem(def.WorkspaceID, current, system)
}

func isOwnedByCurrentWorkspace(resourceWorkspaceID *uuid.UUID, current *domain.Workspace) bool {
	return current != nil && resourceWorkspaceID != nil && *resourceWorkspaceID == current.ID
}

func templateVisibleInWorkspace(tpl *domain.Template, current *domain.Workspace, system *domain.Workspace) bool {
	if tpl == nil {
		return false
	}
	if current == nil {
		return tpl.WorkspaceID == nil
	}
	if isOwnedByCurrentWorkspace(tpl.WorkspaceID, current) {
		return true
	}
	return tpl.WorkspaceID != nil && system != nil && *tpl.WorkspaceID == system.ID
}

func writePolicyForbidden(c *echo.Context, code, message string) error {
	return response.WriteError(c, 403, code, message)
}

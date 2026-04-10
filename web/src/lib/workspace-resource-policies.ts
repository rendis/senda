import {
  SYSTEM_WORKSPACE_CODE,
  type OwnerScope,
  type ScopeContext,
  type ScopeLevel,
} from "../types/api.ts";
import type { WorkspacePolicies } from "../types/settings.ts";

type TemplateResourceLike = {
  owner_scope?: OwnerScope;
  inherited_from_system?: boolean;
  is_fork?: boolean;
  workspace_id?: string;
  scope_level?: ScopeLevel;
};

type TemplateTypeResourceLike = {
  owner_scope?: OwnerScope;
  inherited_from_system?: boolean;
  workspace_id?: string;
  scope_level?: ScopeLevel;
};

type InjectorResourceLike = {
  owner_scope?: OwnerScope;
  inherited_from_system?: boolean;
  workspace_id?: string;
  scope_level?: ScopeLevel;
};

export type ResourceStateBadge =
  | "local"
  | "defaultSystem"
  | "forkedFromDefault"
  | "readOnly"
  | "workspace"
  | "global";

export type ResourceDisplayScope = ScopeLevel | "system";

type ResourceState = {
  badges: ResourceStateBadge[];
  readOnly: boolean;
};

type TemplateCatalogState = {
  canCreateTemplateTypes: boolean;
  canCreateTemplates: boolean;
};

type TemplateTypeManagementState = ResourceState & {
  canEdit: boolean;
  canDelete: boolean;
};

type TemplateManagementState = ResourceState & {
  canFork: boolean;
  canManageVersions: boolean;
  canEditMetadata: boolean;
  versionPrimaryAction: "open" | "edit";
};

type InjectorManagementState = ResourceState & {
  canEdit: boolean;
  canDelete: boolean;
};

export const DEFAULT_WORKSPACE_POLICIES: WorkspacePolicies = {
  allow_workspace_local_templates: true,
  allow_workspace_inherited_template_forks: true,
  allow_workspace_local_injectors: true,
};

export function resolveWorkspacePolicies(
  policies?: Partial<WorkspacePolicies> | null,
): WorkspacePolicies {
  return {
    allow_workspace_local_templates:
      policies?.allow_workspace_local_templates ??
      DEFAULT_WORKSPACE_POLICIES.allow_workspace_local_templates,
    allow_workspace_inherited_template_forks:
      policies?.allow_workspace_inherited_template_forks ??
      DEFAULT_WORKSPACE_POLICIES.allow_workspace_inherited_template_forks,
    allow_workspace_local_injectors:
      policies?.allow_workspace_local_injectors ??
      DEFAULT_WORKSPACE_POLICIES.allow_workspace_local_injectors,
  };
}

export function canShowGlobalSettings(scope: ScopeContext): boolean {
  return scope.level === "global";
}

export function canManageSystemWorkspacePolicies(scope: ScopeContext): boolean {
  return (
    scope.level === "workspace" && scope.workspaceCode === SYSTEM_WORKSPACE_CODE
  );
}

export function isWorkspaceScope(scope: ScopeContext): boolean {
  return scope.level === "workspace";
}

export function isSystemWorkspaceScope(scope: ScopeContext): boolean {
  return canManageSystemWorkspacePolicies(scope);
}

export function getTemplateCatalogState(
  scope: ScopeContext,
  policies?: Partial<WorkspacePolicies> | null,
): TemplateCatalogState {
  const workspaceScope = isWorkspaceScope(scope);
  const systemScope = isSystemWorkspaceScope(scope);

  if (!workspaceScope || systemScope) {
    return {
      canCreateTemplateTypes: true,
      canCreateTemplates: true,
    };
  }

  if (!policies) {
    return {
      canCreateTemplateTypes: false,
      canCreateTemplates: false,
    };
  }

  const resolved = resolveWorkspacePolicies(policies);
  return {
    canCreateTemplateTypes: resolved.allow_workspace_local_templates,
    canCreateTemplates: resolved.allow_workspace_local_templates,
  };
}

export function getTemplateTypeManagementState(
  scope: ScopeContext,
  resource?: TemplateTypeResourceLike | null,
  policies?: Partial<WorkspacePolicies> | null,
): TemplateTypeManagementState {
  const base = getBaseResourceState(resource);

  if (!resource) {
    return {
      ...base,
      canEdit: false,
      canDelete: false,
    };
  }

  const resolved = resolveWorkspacePolicies(policies);
  if (!isWorkspaceScope(scope) || isSystemWorkspaceScope(scope)) {
    return {
      ...base,
      canEdit: true,
      canDelete: true,
    };
  }

  if (resource.owner_scope !== "local") {
    return {
      ...appendReadOnlyBadge(base),
      canEdit: false,
      canDelete: false,
    };
  }

  if (!policies) {
    return {
      ...appendReadOnlyBadge(base),
      canEdit: false,
      canDelete: false,
    };
  }

  const allowed = resolved.allow_workspace_local_templates;
  return {
    ...withConditionalReadOnly(base, !allowed),
    canEdit: allowed,
    canDelete: allowed,
  };
}

export function getTemplateManagementState(
  scope: ScopeContext,
  resource?: TemplateResourceLike | null,
  policies?: Partial<WorkspacePolicies> | null,
): TemplateManagementState {
  const base = getBaseResourceState(resource);

  if (!resource) {
    return {
      ...base,
      canFork: false,
      canManageVersions: false,
      canEditMetadata: false,
      versionPrimaryAction: "open" as const,
    };
  }

  const resolved = resolveWorkspacePolicies(policies);
  if (!isWorkspaceScope(scope) || isSystemWorkspaceScope(scope)) {
    return {
      ...base,
      canFork: false,
      canManageVersions: true,
      canEditMetadata: true,
      versionPrimaryAction: "edit" as const,
    };
  }

  if (resource.is_fork) {
    if (!policies) {
      return {
        badges: ["forkedFromDefault", "readOnly"],
        readOnly: true,
        canFork: false,
        canManageVersions: false,
        canEditMetadata: false,
        versionPrimaryAction: "open" as const,
      };
    }
    const allowed = resolved.allow_workspace_inherited_template_forks;
    return {
      badges: ["forkedFromDefault"],
      readOnly: false,
      canFork: false,
      canManageVersions: allowed,
      canEditMetadata: false,
      versionPrimaryAction: allowed ? ("edit" as const) : ("open" as const),
    };
  }

  if (resource.owner_scope === "system" || resource.inherited_from_system) {
    return {
      ...appendReadOnlyBadge(base),
      canFork: Boolean(policies && resolved.allow_workspace_inherited_template_forks),
      canManageVersions: false,
      canEditMetadata: false,
      versionPrimaryAction: "open" as const,
    };
  }

  if (!policies) {
    return {
      ...appendReadOnlyBadge(base),
      canFork: false,
      canManageVersions: false,
      canEditMetadata: false,
      versionPrimaryAction: "open" as const,
    };
  }

  const allowed = resolved.allow_workspace_local_templates;
  return {
    ...withConditionalReadOnly(base, !allowed),
    canFork: false,
    canManageVersions: allowed,
    canEditMetadata: allowed,
    versionPrimaryAction: allowed ? ("edit" as const) : ("open" as const),
  };
}

export function getInjectorManagementState(
  scope: ScopeContext,
  resource?: InjectorResourceLike | null,
  policies?: Partial<WorkspacePolicies> | null,
): InjectorManagementState {
  const base = getBaseResourceState(resource);

  if (!resource) {
    return {
      ...base,
      canEdit: false,
      canDelete: false,
    };
  }

  const resolved = resolveWorkspacePolicies(policies);
  if (!isWorkspaceScope(scope) || isSystemWorkspaceScope(scope)) {
    return {
      ...base,
      canEdit: true,
      canDelete: true,
    };
  }

  if (resource.owner_scope !== "local") {
    return {
      ...appendReadOnlyBadge(base),
      canEdit: false,
      canDelete: false,
    };
  }

  if (!policies) {
    return {
      ...appendReadOnlyBadge(base),
      canEdit: false,
      canDelete: false,
    };
  }

  const allowed = resolved.allow_workspace_local_injectors;
  return {
    ...withConditionalReadOnly(base, !allowed),
    canEdit: allowed,
    canDelete: allowed,
  };
}

function getBaseResourceState(
  resource?: {
    owner_scope?: OwnerScope;
    inherited_from_system?: boolean;
    is_fork?: boolean;
  } | null,
): ResourceState {
  if (resource?.is_fork) {
    return {
      badges: ["forkedFromDefault"],
      readOnly: false,
    };
  }

  if (resource?.owner_scope === "local") {
    return {
      badges: ["local"],
      readOnly: false,
    };
  }

  if (resource?.owner_scope === "system" || resource?.inherited_from_system) {
    return {
      badges: ["defaultSystem"],
      readOnly: true,
    };
  }

  if (resource?.owner_scope === "workspace") {
    return {
      badges: ["workspace"],
      readOnly: false,
    };
  }

  if (resource?.owner_scope === "global") {
    return {
      badges: ["global"],
      readOnly: false,
    };
  }

  return {
    badges: [],
    readOnly: false,
  };
}

export function resolveResourceDisplayScope(
  resource?: {
    owner_scope?: OwnerScope;
    inherited_from_system?: boolean;
    workspace_id?: string;
    scope_level?: ScopeLevel;
  } | null,
  fallbackScope: ScopeLevel = "global",
): ResourceDisplayScope {
  if (resource?.owner_scope === "system" || resource?.inherited_from_system) {
    return "system";
  }

  if (resource?.owner_scope === "local" || resource?.owner_scope === "workspace") {
    return "workspace";
  }

  if (resource?.owner_scope === "global") {
    return "global";
  }

  if (resource?.scope_level) {
    return resource.scope_level;
  }

  if (resource?.workspace_id) {
    return "workspace";
  }

  return fallbackScope;
}

function appendReadOnlyBadge(state: ResourceState): ResourceState {
  if (state.badges.includes("readOnly")) {
    return state;
  }

  return {
    ...state,
    readOnly: true,
    badges: [...state.badges, "readOnly"],
  };
}

function withConditionalReadOnly(
  state: ResourceState,
  readOnly: boolean,
): ResourceState {
  return readOnly ? appendReadOnlyBadge(state) : state;
}

import type { ScopeContext } from "../types/api";
import type { ExternalIntegrationCapabilities } from "../types/settings";

const SYSTEM_WORKSPACE_CODE = "_system";
const SYSTEM_WORKSPACE_LABEL = "Default";
const SYSTEM_WORKSPACE_SCOPE_LABEL = "Default scope";

function isSystemWorkspaceCode(code?: string | null) {
  return code === SYSTEM_WORKSPACE_CODE;
}

export interface ExternalTemplateBuilderContext {
  profileSlug: string;
  scope: ScopeContext;
  fallbackToSystem?: boolean;
  session?: ExternalEmbedSessionState | null;
  accessDenied?: boolean;
}

export interface ExternalEmbedSessionState {
  read_only: boolean;
  effective_workspace_code: string;
  permissions: ExternalIntegrationCapabilities;
}

export interface ExternalTemplateBuilderViewState {
  readOnly: boolean;
  readOnlyReason?: string;
  workspaceLabel: string;
  scopeLabel: string;
  canEdit: boolean;
  canPublish: boolean;
  canTestSend: boolean;
  accessDenied: boolean;
}

export function resolveExternalTemplateBuilderViewState(
  context: ExternalTemplateBuilderContext,
): ExternalTemplateBuilderViewState {
  const fallbackToSystem =
    context.session?.read_only ??
    context.fallbackToSystem ??
    isSystemWorkspaceCode(context.scope.workspaceCode);
  const effectiveWorkspaceCode =
    context.session?.effective_workspace_code ?? context.scope.workspaceCode;
  const accessDenied = context.accessDenied ?? false;

  if (accessDenied) {
    return {
      readOnly: true,
      workspaceLabel: effectiveWorkspaceCode ?? SYSTEM_WORKSPACE_CODE,
      scopeLabel: "Workspace",
      canEdit: false,
      canPublish: false,
      canTestSend: false,
      accessDenied: true,
    };
  }

  if (fallbackToSystem) {
    return {
      readOnly: true,
      readOnlyReason: "Resolved to the tenant Default workspace fallback.",
      workspaceLabel: SYSTEM_WORKSPACE_LABEL,
      scopeLabel: SYSTEM_WORKSPACE_SCOPE_LABEL,
      canEdit: false,
      canPublish: false,
      canTestSend: false,
      accessDenied: false,
    };
  }

  const permissions = context.session?.permissions;
  return {
    readOnly: false,
    workspaceLabel: effectiveWorkspaceCode ?? SYSTEM_WORKSPACE_CODE,
    scopeLabel: "Workspace",
    canEdit: permissions?.edit_versions ?? true,
    canPublish: permissions?.publish_versions ?? true,
    canTestSend: permissions?.test_send ?? true,
    accessDenied: false,
  };
}

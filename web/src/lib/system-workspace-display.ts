export const SYSTEM_WORKSPACE_LABEL = "System";
const INTERNAL_SYSTEM_WORKSPACE_CODE = "_system";

export function isSystemWorkspaceLike(input: {
  code?: string | null;
  is_system?: boolean | null;
}) {
  return Boolean(input.is_system) || input.code === INTERNAL_SYSTEM_WORKSPACE_CODE;
}

export function getWorkspaceDisplayName(input: {
  code?: string | null;
  name?: string | null;
  is_system?: boolean | null;
}) {
  if (isSystemWorkspaceLike(input)) {
    return SYSTEM_WORKSPACE_LABEL;
  }

  return input.name?.trim() || input.code || "";
}

export function getWorkspaceDisplayCode(input: {
  code?: string | null;
  is_system?: boolean | null;
}) {
  if (isSystemWorkspaceLike(input)) {
    return SYSTEM_WORKSPACE_LABEL;
  }

  return input.code || "";
}

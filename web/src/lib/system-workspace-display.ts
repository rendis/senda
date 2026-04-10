import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

export const SYSTEM_WORKSPACE_LABEL = "Default";
export const SYSTEM_WORKSPACE_SCOPE_LABEL = "Default scope";

export function isSystemWorkspaceCode(code?: string | null) {
  return code === SYSTEM_WORKSPACE_CODE;
}

export function isSystemWorkspaceLike(input: {
  code?: string | null;
  is_system?: boolean | null;
}) {
  return Boolean(input.is_system) || isSystemWorkspaceCode(input.code);
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

export function getTenantSystemPath(tenantCode: string, suffix = "") {
  return `/t/${tenantCode}/w/${SYSTEM_WORKSPACE_CODE}${suffix}`;
}

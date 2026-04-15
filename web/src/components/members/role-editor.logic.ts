import type { MemberRoleDetail } from "@/types/members-ext";
import type { Role, ScopeContext } from "@/types/api";

const OPTIMISTIC_ROLE_PREFIX = "optimistic-role:";

export function hasSingleFixedRole(allowedRoles: Role[]): boolean {
  return allowedRoles.length <= 1;
}

export function getRoleEditorSubmitLabel(currentRole?: Role): string {
  return currentRole ? "Save" : "Grant access";
}

export function getRoleEditorState({
  allowedRoles,
  currentRole,
  selectedRole,
}: {
  allowedRoles: Role[];
  currentRole?: Role;
  selectedRole?: Role;
}): {
  roleOptions: Role[];
  selectDisabled: boolean;
  submitDisabled: boolean;
  helperText: string;
} {
  const roleOptions = [...allowedRoles];
  const singleFixedRole = hasSingleFixedRole(roleOptions);
  const submitDisabled = !selectedRole || selectedRole === currentRole;

  return {
    roleOptions,
    selectDisabled: singleFixedRole,
    submitDisabled,
    helperText:
      singleFixedRole && currentRole
        ? "This scope has a single valid role. Remove access to revoke it."
        : "Select the single local role for the current scope.",
  };
}

export function replaceScopedRoleLocally(
  roles: MemberRoleDetail[],
  next: {
    role: Role;
    scope: ScopeContext;
  },
  nowIso = new Date().toISOString(),
): MemberRoleDetail[] {
  const remaining = roles.filter((role) => !matchesScope(role, next.scope));

  return [
    ...remaining,
    {
      id: `${OPTIMISTIC_ROLE_PREFIX}${buildScopeKey(next.scope)}:${next.role}`,
      member_id: "",
      role: next.role,
      scope_type: next.scope.level,
      tenant_code: next.scope.tenantCode,
      workspace_code: next.scope.workspaceCode,
      created_at: nowIso,
    },
  ];
}

function matchesScope(role: MemberRoleDetail, scope: ScopeContext): boolean {
  switch (scope.level) {
    case "tenant":
      return (
        role.scope_type === "tenant" &&
        (role.tenant_code == null || role.tenant_code === scope.tenantCode)
      );
    case "workspace":
      return (
        role.scope_type === "workspace" &&
        (role.tenant_code == null || role.tenant_code === scope.tenantCode) &&
        (role.workspace_code == null || role.workspace_code === scope.workspaceCode)
      );
    case "global":
    default:
      return role.scope_type === "global";
  }
}

function buildScopeKey(scope: ScopeContext): string {
  switch (scope.level) {
    case "tenant":
      return `tenant:${scope.tenantCode ?? "*"}`;
    case "workspace":
      return `workspace:${scope.tenantCode ?? "*"}:${scope.workspaceCode ?? "*"}`;
    case "global":
    default:
      return "global";
  }
}

import type { ScopeContext, Role, ScopeLevel } from "../types/api.ts";
import type {
  InviteMemberRequest,
  MemberRoleDetail,
  MemberWithRoles,
  ReplaceMemberRoleRequest,
} from "../types/members-ext.ts";

export interface MemberRowAction {
  kind: "change-role" | "revoke-access";
  label: string;
  destructive: boolean;
}

export interface RevokeAccessDialogCopy {
  title: string;
  description: string;
  confirmLabel: string;
}

export type InviteMemberOutcome =
  | {
      status: "success";
      member: MemberWithRoles;
    }
  | {
      status: "needs-role-retry";
      member: MemberWithRoles;
      error: unknown;
    };

export function getAllowedMemberRolesForScope(scopeLevel: ScopeLevel): Role[] {
  switch (scopeLevel) {
    case "tenant":
      return ["tenant_admin"];
    case "workspace":
      return ["workspace_viewer", "workspace_editor", "workspace_admin"];
    case "global":
    default:
      return ["superadmin"];
  }
}

export function buildMembersPath(scope: ScopeContext): string {
  switch (scope.level) {
    case "tenant":
      return `manage/tenants/${scope.tenantCode}/members`;
    case "workspace":
      return `manage/environments/${scope.environment ?? "prod"}/tenants/${scope.tenantCode}/workspaces/${scope.workspaceCode}/members`;
    case "global":
    default:
      return "manage/members";
  }
}

export function buildMemberAccessPath(scope: ScopeContext, memberId: string): string {
  return `${buildMembersPath(scope)}/${memberId}/access`;
}

export function buildMemberRolePath(
  scope: ScopeContext,
  memberId: string,
): string {
  return `${buildMembersPath(scope)}/${memberId}/role`;
}

export function buildInviteMemberRequest(
  scopeLevel: ScopeLevel,
  data: { email: string; display_name?: string; role: Role },
): InviteMemberRequest {
  if (scopeLevel === "global") {
    return {
      email: data.email,
      display_name: data.display_name,
    };
  }

  return data;
}

export async function inviteMemberInScope({
  scopeLevel,
  formData,
  inviteMember,
  replaceMemberRole,
}: {
  scopeLevel: ScopeLevel;
  formData: { email: string; display_name?: string; role: Role };
  inviteMember: (data: InviteMemberRequest) => Promise<MemberWithRoles>;
  replaceMemberRole: (request: {
    memberId: string;
    data: ReplaceMemberRoleRequest;
  }) => Promise<unknown>;
}): Promise<InviteMemberOutcome> {
  if (scopeLevel !== "global") {
    const member = await inviteMember(formData);
    return {
      status: "success",
      member,
    };
  }

  const member = await inviteMember(buildInviteMemberRequest(scopeLevel, formData));

  try {
    await replaceMemberRole({
      memberId: member.id,
      data: { role: formData.role, scope_type: scopeLevel },
    });
    return {
      status: "success",
      member,
    };
  } catch (error) {
    return {
      status: "needs-role-retry",
      member,
      error,
    };
  }
}

export function getMemberRowActions(scopeLevel: ScopeLevel): MemberRowAction[] {
  if (scopeLevel === "workspace") {
    return [
      {
        kind: "change-role",
        label: "Change role",
        destructive: false,
      },
      {
        kind: "revoke-access",
        label: "Remove access in current scope",
        destructive: true,
      },
    ];
  }

  return [
    {
      kind: "revoke-access",
      label: "Remove access in current scope",
      destructive: true,
    },
  ];
}

export function hasMemberAccessInScope(
  member: Pick<MemberWithRoles, "roles">,
  scope: ScopeContext,
): boolean {
  return getMemberRolesInScope(member, scope).length > 0;
}

export function getMemberRolesInScope(
  member: Pick<MemberWithRoles, "roles">,
  scope: ScopeContext,
): MemberRoleDetail[] {
  return (member.roles ?? [])
    .filter((role) => roleMatchesScope(role, scope))
    .sort(compareRolePriorityDesc);
}

export function getPrimaryMemberRoleInScope(
  member: Pick<MemberWithRoles, "roles">,
  scope: ScopeContext,
): MemberRoleDetail | undefined {
  return getMemberRolesInScope(member, scope)[0];
}

export function buildRevokeAccessDialogCopy({
  memberEmail,
  scopeLabel,
}: {
  memberEmail: string;
  scopeLabel: string;
}): RevokeAccessDialogCopy {
  return {
    title: "Remove access",
    description: `Remove access for ${memberEmail} in ${scopeLabel}? This removes access in the current scope.`,
    confirmLabel: "Remove access",
  };
}

export function removeMemberFromCachedPages<
  T extends { id: string },
  TPage extends { items: T[] },
>(data: { pages: TPage[]; pageParams: unknown[] } | undefined, memberId: string) {
  if (!data) return data;

  return {
    ...data,
    pages: data.pages.map((page) => ({
      ...page,
      items: page.items.filter((member) => member.id !== memberId),
    })),
  };
}

export async function performNoContentRequest(
  request: () => Promise<unknown> | unknown,
): Promise<void> {
  await request();
}

function roleMatchesScope(role: MemberRoleDetail, scope: ScopeContext): boolean {
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

const ROLE_PRIORITY: Record<Role, number> = {
  superadmin: 5,
  tenant_admin: 4,
  workspace_admin: 3,
  workspace_editor: 2,
  workspace_viewer: 1,
};

function compareRolePriorityDesc(left: MemberRoleDetail, right: MemberRoleDetail): number {
  return ROLE_PRIORITY[right.role] - ROLE_PRIORITY[left.role];
}

import type { Role, ScopeLevel } from "./api";

/** Member with roles (extended detail) */
export interface MemberWithRoles {
  id: string;
  email: string;
  display_name: string;
  roles: MemberRoleDetail[];
  created_at: string;
  updated_at: string;
}

/** Detailed role assignment */
export interface MemberRoleDetail {
  id: string;
  member_id: string;
  role: Role;
  scope_type: ScopeLevel;
  tenant_id?: string;
  workspace_id?: string;
  created_at: string;
}

/** Invite member request */
export interface InviteMemberRequest {
  email: string;
  display_name?: string;
}

/** Add role to member request */
export interface AddMemberRoleRequest {
  role: Role;
  scope_type: ScopeLevel;
  tenant_id?: string;
  workspace_id?: string;
}

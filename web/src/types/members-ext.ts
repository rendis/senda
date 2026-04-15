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
  tenant_code?: string;
  workspace_code?: string;
  created_at: string;
}

/** Invite member request */
export interface InviteMemberRequest {
  email: string;
  display_name?: string;
  role?: Role;
}

/** Replace the single local role for the current scope */
export interface ReplaceMemberRoleRequest {
  role: Role;
  scope_type: ScopeLevel;
  tenant_id?: string;
  workspace_id?: string;
}

/** @deprecated Use ReplaceMemberRoleRequest */
export type AddMemberRoleRequest = ReplaceMemberRoleRequest;

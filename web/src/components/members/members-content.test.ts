import assert from "node:assert/strict";
import test from "node:test";

import {
  buildRevokeAccessDialogCopy,
  getMemberRolesInScope,
  getMemberRowActions,
  getPrimaryMemberRoleInScope,
  hasMemberAccessInScope,
} from "../../hooks/members-mgmt-logic.ts";

const CREATED_AT = "2026-04-14T00:00:00Z";
const MEMBER_ID = "member-1";
const TENANT_CODE = "acme";
const WORKSPACE_CODE = "main";
const TENANT_SCOPE_CODE = "system-test-corp";
const WORKSPACE_SCOPE_CODE = "system-main";
const TENANT_ROLE_ID = "tenant-role";
const WORKSPACE_ROLE_ID = "workspace-role";
const REVOKE_ACCESS_ACTION = "revoke-access";
const WORKSPACE_SCOPE = "workspace";

test("member row actions stay simple and only allow changing roles in workspace scope", () => {
  assert.deepEqual(
    getMemberRowActions("global").map((action) => action.kind),
    [REVOKE_ACCESS_ACTION],
  );
  assert.deepEqual(
    getMemberRowActions("tenant").map((action) => action.kind),
    [REVOKE_ACCESS_ACTION],
  );
  assert.deepEqual(
    getMemberRowActions(WORKSPACE_SCOPE).map((action) => action.kind),
    ["change-role", REVOKE_ACCESS_ACTION],
  );
  assert.equal(
    getMemberRowActions(WORKSPACE_SCOPE).filter((action) => action.destructive).length,
    1,
  );
  assert.equal(
    getMemberRowActions(WORKSPACE_SCOPE).find((action) => action.kind === "change-role")
      ?.label,
    "Change role",
  );
});

test("revoke access dialog copy references the current scope instead of a role", () => {
  const copy = buildRevokeAccessDialogCopy({
    memberEmail: "admin@senda.dev",
    scopeLabel: "tenant \"acme\"",
  });

  assert.equal(copy.title, "Remove access");
  assert.equal(copy.confirmLabel, "Remove access");
  assert.match(copy.description, /admin@senda\.dev/);
  assert.match(copy.description, /tenant "acme"/);
  assert.doesNotMatch(copy.description, /role/i);
});

test("revoke access is only available for members with a global grant in global scope", () => {
  assert.equal(
    hasMemberAccessInScope(
      {
        roles: [
          {
            id: "role-workspace",
            member_id: MEMBER_ID,
            role: "workspace_admin",
            scope_type: "workspace",
            tenant_code: TENANT_CODE,
            workspace_code: WORKSPACE_CODE,
            created_at: CREATED_AT,
          },
        ],
      },
      { level: "global" },
    ),
    false,
  );

  assert.equal(
    hasMemberAccessInScope(
      {
        roles: [
          {
            id: "role-global",
            member_id: MEMBER_ID,
            role: "superadmin",
            scope_type: "global",
            created_at: CREATED_AT,
          },
        ],
      },
      { level: "global" },
    ),
    true,
  );
});

test("revoke access is only available when the member matches the current tenant or workspace", () => {
  const roles = [
    {
      id: TENANT_ROLE_ID,
      member_id: MEMBER_ID,
      role: "tenant_admin" as const,
      scope_type: "tenant" as const,
      tenant_code: TENANT_CODE,
      created_at: CREATED_AT,
    },
    {
      id: WORKSPACE_ROLE_ID,
      member_id: MEMBER_ID,
      role: "workspace_admin" as const,
      scope_type: "workspace" as const,
      tenant_code: TENANT_CODE,
      workspace_code: WORKSPACE_CODE,
      created_at: CREATED_AT,
    },
  ];

  assert.equal(
    hasMemberAccessInScope({ roles }, { level: "tenant", tenantCode: TENANT_CODE }),
    true,
  );
  assert.equal(
    hasMemberAccessInScope({ roles }, { level: "tenant", tenantCode: "other" }),
    false,
  );
  assert.equal(
    hasMemberAccessInScope(
      { roles },
      {
        level: "workspace",
        tenantCode: TENANT_CODE,
        workspaceCode: WORKSPACE_CODE,
      },
    ),
    true,
  );
  assert.equal(
    hasMemberAccessInScope(
      { roles },
      { level: "workspace", tenantCode: "acme", workspaceCode: "secondary" },
    ),
    false,
  );
});

test("scoped member payloads stay visible even when tenant and workspace codes are omitted", () => {
  const tenantScopedMember = {
    roles: [
      {
        id: TENANT_ROLE_ID,
        member_id: MEMBER_ID,
        role: "tenant_admin" as const,
        scope_type: "tenant" as const,
        created_at: CREATED_AT,
      },
    ],
  };
  const workspaceScopedMember = {
    roles: [
      {
        id: WORKSPACE_ROLE_ID,
        member_id: MEMBER_ID,
        role: "workspace_admin" as const,
        scope_type: "workspace" as const,
        created_at: CREATED_AT,
      },
    ],
  };

  assert.equal(
    hasMemberAccessInScope(
      tenantScopedMember,
      { level: "tenant", tenantCode: TENANT_SCOPE_CODE },
    ),
    true,
  );
  assert.equal(
    hasMemberAccessInScope(
      workspaceScopedMember,
      {
        level: "workspace",
        tenantCode: TENANT_SCOPE_CODE,
        workspaceCode: WORKSPACE_SCOPE_CODE,
      },
    ),
    true,
  );
  assert.equal(
    getPrimaryMemberRoleInScope(
      tenantScopedMember,
      { level: "tenant", tenantCode: TENANT_SCOPE_CODE },
    )?.id,
    TENANT_ROLE_ID,
  );
  assert.equal(
    getPrimaryMemberRoleInScope(workspaceScopedMember, {
      level: "workspace",
      tenantCode: TENANT_SCOPE_CODE,
      workspaceCode: WORKSPACE_SCOPE_CODE,
    })?.id,
    WORKSPACE_ROLE_ID,
  );
});

test("members table derives the visible role and scope from the current scope", () => {
  const member = {
    roles: [
      {
        id: "workspace-role",
        member_id: MEMBER_ID,
        role: "workspace_admin" as const,
        scope_type: "workspace" as const,
        tenant_code: TENANT_CODE,
        workspace_code: WORKSPACE_CODE,
        created_at: CREATED_AT,
      },
      {
        id: "tenant-role",
        member_id: MEMBER_ID,
        role: "tenant_admin" as const,
        scope_type: "tenant" as const,
        tenant_code: TENANT_CODE,
        created_at: CREATED_AT,
      },
      {
        id: "global-role",
        member_id: MEMBER_ID,
        role: "superadmin" as const,
        scope_type: "global" as const,
        created_at: CREATED_AT,
      },
    ],
  };

  assert.equal(
    getPrimaryMemberRoleInScope(member, { level: "global" })?.id,
    "global-role",
  );
  assert.equal(
    getPrimaryMemberRoleInScope(member, {
      level: "tenant",
      tenantCode: TENANT_CODE,
    })?.id,
    "tenant-role",
  );
  assert.equal(
    getPrimaryMemberRoleInScope(member, {
      level: "workspace",
      tenantCode: TENANT_CODE,
      workspaceCode: WORKSPACE_CODE,
    })?.id,
    "workspace-role",
  );
});

test("legacy multi-role data falls back to the highest-priority local role in the current scope", () => {
  const member = {
    roles: [
      {
        id: "viewer-role",
        member_id: MEMBER_ID,
        role: "workspace_viewer" as const,
        scope_type: "workspace" as const,
        tenant_code: TENANT_CODE,
        workspace_code: WORKSPACE_CODE,
        created_at: CREATED_AT,
      },
      {
        id: "admin-role",
        member_id: MEMBER_ID,
        role: "workspace_admin" as const,
        scope_type: "workspace" as const,
        tenant_code: TENANT_CODE,
        workspace_code: WORKSPACE_CODE,
        created_at: CREATED_AT,
      },
      {
        id: "editor-role",
        member_id: MEMBER_ID,
        role: "workspace_editor" as const,
        scope_type: "workspace" as const,
        tenant_code: TENANT_CODE,
        workspace_code: WORKSPACE_CODE,
        created_at: CREATED_AT,
      },
    ],
  };

  assert.equal(
    getPrimaryMemberRoleInScope(member, {
      level: "workspace",
      tenantCode: TENANT_CODE,
      workspaceCode: WORKSPACE_CODE,
    })?.id,
    "admin-role",
  );
});

test("members outside the current scope are excluded from scoped roles", () => {
  const member = {
    roles: [
      {
        id: "tenant-role",
        member_id: MEMBER_ID,
        role: "tenant_admin" as const,
        scope_type: "tenant" as const,
        tenant_code: TENANT_CODE,
        created_at: CREATED_AT,
      },
      {
        id: "workspace-role",
        member_id: MEMBER_ID,
        role: "workspace_admin" as const,
        scope_type: "workspace" as const,
        tenant_code: TENANT_CODE,
        workspace_code: WORKSPACE_CODE,
        created_at: CREATED_AT,
      },
    ],
  };

  assert.deepEqual(
    getMemberRolesInScope(member, { level: "global" }).map((role) => role.id),
    [],
  );
  assert.deepEqual(
    getMemberRolesInScope(member, { level: "tenant", tenantCode: TENANT_CODE }).map(
      (role) => role.id,
    ),
    ["tenant-role"],
  );
  assert.deepEqual(
    getMemberRolesInScope(member, {
      level: "workspace",
      tenantCode: TENANT_CODE,
      workspaceCode: WORKSPACE_CODE,
    }).map((role) => role.id),
    ["workspace-role"],
  );
});

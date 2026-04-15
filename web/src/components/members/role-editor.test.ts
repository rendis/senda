import assert from "node:assert/strict";
import test from "node:test";

import {
  getRoleEditorState,
  getRoleEditorSubmitLabel,
  hasSingleFixedRole,
  replaceScopedRoleLocally,
} from "./role-editor.logic.ts";

const CREATED_AT = "2026-04-14T00:00:00Z";

test("workspace role editor keeps all valid roles and disables save when nothing changes", () => {
  const state = getRoleEditorState({
    allowedRoles: [
      "workspace_viewer",
      "workspace_editor",
      "workspace_admin",
    ],
    currentRole: "workspace_editor",
    selectedRole: "workspace_editor",
  });

  assert.deepEqual(state.roleOptions, [
    "workspace_viewer",
    "workspace_editor",
    "workspace_admin",
  ]);
  assert.equal(state.selectDisabled, false);
  assert.equal(state.submitDisabled, true);
  assert.equal(state.helperText, "Select the single local role for the current scope.");
});

test("single-role scopes keep the selector stable and disable it when there is no alternative", () => {
  const state = getRoleEditorState({
    allowedRoles: ["superadmin"],
    currentRole: "superadmin",
    selectedRole: "superadmin",
  });

  assert.deepEqual(state.roleOptions, ["superadmin"]);
  assert.equal(state.selectDisabled, true);
  assert.equal(state.submitDisabled, true);
  assert.equal(
    state.helperText,
    "This scope has a single valid role. Remove access to revoke it.",
  );
  assert.equal(hasSingleFixedRole(["superadmin"]), true);
});

test("single-role scopes still allow recovery when access is missing", () => {
  const state = getRoleEditorState({
    allowedRoles: ["superadmin"],
    selectedRole: "superadmin",
  });

  assert.equal(state.selectDisabled, true);
  assert.equal(state.submitDisabled, false);
  assert.equal(getRoleEditorSubmitLabel(undefined), "Grant access");
  assert.equal(getRoleEditorSubmitLabel("workspace_viewer"), "Save");
});

test("local workspace role replacement removes legacy duplicates in the same scope", () => {
  const roles = [
    {
      id: "viewer-role",
      member_id: "member-1",
      role: "workspace_viewer" as const,
      scope_type: "workspace" as const,
      tenant_code: "acme",
      workspace_code: "main",
      created_at: CREATED_AT,
    },
    {
      id: "editor-role",
      member_id: "member-1",
      role: "workspace_editor" as const,
      scope_type: "workspace" as const,
      tenant_code: "acme",
      workspace_code: "main",
      created_at: CREATED_AT,
    },
    {
      id: "other-workspace-role",
      member_id: "member-1",
      role: "workspace_admin" as const,
      scope_type: "workspace" as const,
      tenant_code: "acme",
      workspace_code: "secondary",
      created_at: CREATED_AT,
    },
  ];

  const next = replaceScopedRoleLocally(roles, {
    role: "workspace_admin",
    scope: {
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "main",
    },
  });

  assert.deepEqual(
    next.map((role) => `${role.id}:${role.role}:${role.workspace_code ?? ""}`),
    [
      "other-workspace-role:workspace_admin:secondary",
      "optimistic-role:workspace:acme:main:workspace_admin:workspace_admin:main",
    ],
  );
});

test("local tenant role replacement only touches the current tenant scope", () => {
  const roles = [
    {
      id: "tenant-role",
      member_id: "member-1",
      role: "tenant_admin" as const,
      scope_type: "tenant" as const,
      tenant_code: "acme",
      created_at: CREATED_AT,
    },
    {
      id: "global-role",
      member_id: "member-1",
      role: "superadmin" as const,
      scope_type: "global" as const,
      created_at: CREATED_AT,
    },
  ];

  const next = replaceScopedRoleLocally(roles, {
    role: "tenant_admin",
    scope: {
      level: "tenant",
      tenantCode: "other",
    },
  });

  assert.equal(next.length, 3);
  assert.equal(next.some((role) => role.id === "tenant-role"), true);
  assert.equal(next.some((role) => role.id === "global-role"), true);
});

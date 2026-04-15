import assert from "node:assert/strict";
import test from "node:test";

import {
  buildInviteMemberRequest,
  buildMemberAccessPath,
  buildMemberRolePath,
  buildMembersPath,
  getAllowedMemberRolesForScope,
  getMemberRowActions,
  inviteMemberInScope,
  performNoContentRequest,
  removeMemberFromCachedPages,
} from "./members-mgmt-logic.ts";

const NEW_USER_EMAIL = "new.user@example.com";
const NEW_USER_NAME = "New User";
const TENANT_USER_EMAIL = "tenant.user@example.com";
const TENANT_USER_NAME = "Tenant User";
const TIMESTAMP = "2026-04-14T00:00:00Z";

test("global member invite omits role from create payload", () => {
  const payload = buildInviteMemberRequest("global", {
    email: NEW_USER_EMAIL,
    display_name: NEW_USER_NAME,
    role: "superadmin",
  });

  assert.deepEqual(payload, {
    email: NEW_USER_EMAIL,
    display_name: NEW_USER_NAME,
  });
  assert.equal("role" in payload, false);
});

test("tenant member invite keeps role in create payload", () => {
  const payload = buildInviteMemberRequest("tenant", {
    email: TENANT_USER_EMAIL,
    display_name: TENANT_USER_NAME,
    role: "tenant_admin",
  });

  assert.deepEqual(payload, {
    email: TENANT_USER_EMAIL,
    display_name: TENANT_USER_NAME,
    role: "tenant_admin",
  });
});

test("workspace member invite keeps role in create payload", () => {
  const payload = buildInviteMemberRequest("workspace", {
    email: "workspace.user@example.com",
    display_name: "Workspace User",
    role: "workspace_admin",
  });

  assert.deepEqual(payload, {
    email: "workspace.user@example.com",
    display_name: "Workspace User",
    role: "workspace_admin",
  });
});

test("global member invite returns recovery state when role replacement fails", async () => {
  const member = {
    id: "member-1",
    email: NEW_USER_EMAIL,
    display_name: NEW_USER_NAME,
    roles: [],
    created_at: TIMESTAMP,
    updated_at: TIMESTAMP,
  };
  const replaceRoleCalls: Array<{ memberId: string; data: { role: string; scope_type: string } }> = [];

  const result = await inviteMemberInScope({
    scopeLevel: "global",
    formData: {
      email: NEW_USER_EMAIL,
      display_name: NEW_USER_NAME,
      role: "superadmin",
    },
    inviteMember: async (payload) => {
      assert.deepEqual(payload, {
        email: NEW_USER_EMAIL,
        display_name: NEW_USER_NAME,
      });
      return member;
    },
    replaceMemberRole: async (request) => {
      replaceRoleCalls.push(request);
      throw new Error("role replacement failed");
    },
  });

  assert.equal(result.status, "needs-role-retry");
  assert.equal(result.member, member);
  assert.equal(result.error instanceof Error, true);
  assert.deepEqual(replaceRoleCalls, [
    {
      memberId: "member-1",
      data: { role: "superadmin", scope_type: "global" },
    },
  ]);
});

test("non-global member invite completes in a single request", async () => {
  const member = {
    id: "member-2",
    email: TENANT_USER_EMAIL,
    display_name: TENANT_USER_NAME,
    roles: [],
    created_at: TIMESTAMP,
    updated_at: TIMESTAMP,
  };
  let replaceRoleCalled = false;

  const result = await inviteMemberInScope({
    scopeLevel: "tenant",
    formData: {
      email: TENANT_USER_EMAIL,
      display_name: TENANT_USER_NAME,
      role: "tenant_admin",
    },
    inviteMember: async (payload) => {
      assert.deepEqual(payload, {
        email: TENANT_USER_EMAIL,
        display_name: TENANT_USER_NAME,
        role: "tenant_admin",
      });
      return member;
    },
    replaceMemberRole: async () => {
      replaceRoleCalled = true;
      return member;
    },
  });

  assert.equal(result.status, "success");
  assert.equal(result.member, member);
  assert.equal(replaceRoleCalled, false);
});

test("global members UI only offers supported role combinations", () => {
  assert.deepEqual(getAllowedMemberRolesForScope("global"), ["superadmin"]);
  assert.deepEqual(getAllowedMemberRolesForScope("tenant"), ["tenant_admin"]);
  assert.deepEqual(getAllowedMemberRolesForScope("workspace"), [
    "workspace_viewer",
    "workspace_editor",
    "workspace_admin",
  ]);
});

test("members path follows the current scope", () => {
  assert.equal(buildMembersPath({ level: "global" }), "manage/members");
  assert.equal(
    buildMembersPath({ level: "tenant", tenantCode: "acme" }),
    "manage/tenants/acme/members",
  );
  assert.equal(
    buildMembersPath({
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "main",
      environment: "test",
    }),
    "manage/environments/test/tenants/acme/workspaces/main/members",
  );
});

test("member access path appends access beneath the current members scope", () => {
  assert.equal(
    buildMemberAccessPath({ level: "global" }, "member-1"),
    "manage/members/member-1/access",
  );
  assert.equal(
    buildMemberAccessPath({ level: "tenant", tenantCode: "acme" }, "member-2"),
    "manage/tenants/acme/members/member-2/access",
  );
  assert.equal(
    buildMemberAccessPath(
      {
        level: "workspace",
        tenantCode: "acme",
        workspaceCode: "main",
        environment: "prod",
      },
      "member-3",
    ),
    "manage/environments/prod/tenants/acme/workspaces/main/members/member-3/access",
  );
});

test("member role path uses the singular replace-role endpoint on the same scoped members route", () => {
  assert.equal(
    buildMemberRolePath({ level: "global" }, "member-1"),
    "manage/members/member-1/role",
  );
  assert.equal(
    buildMemberRolePath({ level: "tenant", tenantCode: "acme" }, "member-2"),
    "manage/tenants/acme/members/member-2/role",
  );
  assert.equal(
    buildMemberRolePath(
      {
        level: "workspace",
        tenantCode: "acme",
        workspaceCode: "main",
        environment: "test",
      },
      "member-3",
    ),
    "manage/environments/test/tenants/acme/workspaces/main/members/member-3/role",
  );
});

test("member row actions expose singular role UX by scope", () => {
  assert.deepEqual(
    getMemberRowActions("global").map((action) => action.kind),
    ["revoke-access"],
  );
  assert.deepEqual(
    getMemberRowActions("tenant").map((action) => action.kind),
    ["revoke-access"],
  );
  assert.deepEqual(
    getMemberRowActions("workspace").map((action) => action.kind),
    ["change-role", "revoke-access"],
  );
});

test("cached member pages drop the revoked member from every loaded page", () => {
  const current = {
    pages: [
      {
        items: [
          { id: "member-1", email: "one@example.com" },
          { id: "member-2", email: "two@example.com" },
        ],
      },
      {
        items: [{ id: "member-1", email: "one@example.com" }],
      },
    ],
    pageParams: [undefined, "cursor-2"],
  };

  const next = removeMemberFromCachedPages(current, "member-1");

  assert.deepEqual(next, {
    pages: [
      {
        items: [{ id: "member-2", email: "two@example.com" }],
      },
      {
        items: [],
      },
    ],
    pageParams: [undefined, "cursor-2"],
  });
  assert.deepEqual(current.pages[0].items[0], {
    id: "member-1",
    email: "one@example.com",
  });
});

test("no-content requests resolve without trying to parse a response body", async () => {
  let called = 0;

  await performNoContentRequest(async () => {
    called += 1;
    return undefined;
  });

  assert.equal(called, 1);
});

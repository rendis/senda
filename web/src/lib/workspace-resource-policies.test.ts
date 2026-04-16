import test from "node:test";
import assert from "node:assert/strict";

const {
  DEFAULT_WORKSPACE_POLICIES,
  canManageSystemWorkspacePolicies,
  canShowGlobalSettings,
  getInjectorManagementState,
  getTemplateCatalogState,
  getTemplateManagementState,
  getTemplateTypeManagementState,
  resolveResourceDisplayScope,
  resolveWorkspacePolicies,
} = await import(new URL("./workspace-resource-policies.ts", import.meta.url).href);

test("resolveWorkspacePolicies falls back to compatible defaults", () => {
  assert.deepEqual(resolveWorkspacePolicies(), DEFAULT_WORKSPACE_POLICIES);
  assert.deepEqual(resolveWorkspacePolicies({ allow_workspace_local_templates: false }), {
    allow_workspace_local_templates: false,
    allow_workspace_inherited_template_forks: true,
    allow_workspace_local_injectors: true,
  });
});

test("settings visibility distinguishes global settings from _system workspace policies", () => {
  assert.equal(canShowGlobalSettings({ level: "global" }), true);
  assert.equal(
    canManageSystemWorkspacePolicies({
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "_system",
    }),
    true,
  );
  assert.equal(
    canManageSystemWorkspacePolicies({
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "marketing",
    }),
    false,
  );
});

test("workspace inherited templates stay read-only but can be forked", () => {
  const state = getTemplateManagementState(
    {
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "marketing",
    },
    {
      owner_scope: "system",
      inherited_from_system: true,
      is_fork: false,
    },
    DEFAULT_WORKSPACE_POLICIES,
  );

  assert.equal(state.canFork, true);
  assert.equal(state.canManageVersions, false);
  assert.equal(state.canEditMetadata, false);
  assert.equal(state.versionPrimaryAction, "open");
  assert.deepEqual(state.badges, ["defaultSystem", "readOnly"]);
});

test("forked workspace templates unlock version management without unlocking metadata", () => {
  const state = getTemplateManagementState(
    {
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "marketing",
    },
    {
      owner_scope: "local",
      inherited_from_system: false,
      is_fork: true,
    },
    {
      allow_workspace_local_templates: false,
      allow_workspace_inherited_template_forks: true,
      allow_workspace_local_injectors: true,
    },
  );

  assert.equal(state.canFork, false);
  assert.equal(state.canManageVersions, true);
  assert.equal(state.canEditMetadata, false);
  assert.equal(state.versionPrimaryAction, "edit");
  assert.deepEqual(state.badges, ["forkedFromDefault"]);
});

test("workspace template type creation and local edits obey the local template policy", () => {
  const catalogState = getTemplateCatalogState(
    {
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "marketing",
    },
    {
      allow_workspace_local_templates: false,
      allow_workspace_inherited_template_forks: true,
      allow_workspace_local_injectors: true,
    },
  );

  assert.equal(catalogState.canCreateTemplateTypes, false);

  const typeState = getTemplateTypeManagementState(
    {
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "marketing",
    },
    {
      owner_scope: "local",
      inherited_from_system: false,
    },
    {
      allow_workspace_local_templates: false,
      allow_workspace_inherited_template_forks: true,
      allow_workspace_local_injectors: true,
    },
  );

  assert.equal(typeState.canEdit, false);
  assert.equal(typeState.canDelete, false);
  assert.deepEqual(typeState.badges, ["local", "readOnly"]);
});

test("workspace actions stay conservative when policies are unavailable", () => {
  const scope = {
    level: "workspace" as const,
    tenantCode: "acme",
    workspaceCode: "marketing",
  };

  const catalogState = getTemplateCatalogState(scope, undefined);
  assert.equal(catalogState.canCreateTemplateTypes, false);
  assert.equal(catalogState.canCreateTemplates, false);

  const inheritedTemplateState = getTemplateManagementState(
    scope,
    {
      owner_scope: "system",
      inherited_from_system: true,
      is_fork: false,
    },
    undefined,
  );
  assert.equal(inheritedTemplateState.canFork, false);

  const localInjectorState = getInjectorManagementState(
    scope,
    {
      owner_scope: "local",
      inherited_from_system: false,
    },
    undefined,
  );
  assert.equal(localInjectorState.canEdit, false);
  assert.equal(localInjectorState.canDelete, false);
});

test("injectors expose inherited read-only state and local policy restrictions", () => {
  const inherited = getInjectorManagementState(
    {
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "marketing",
    },
    {
      owner_scope: "system",
      inherited_from_system: true,
    },
    DEFAULT_WORKSPACE_POLICIES,
  );

  assert.equal(inherited.canEdit, false);
  assert.equal(inherited.canDelete, false);
  assert.deepEqual(inherited.badges, ["defaultSystem", "readOnly"]);

  const localBlocked = getInjectorManagementState(
    {
      level: "workspace",
      tenantCode: "acme",
      workspaceCode: "marketing",
    },
    {
      owner_scope: "local",
      inherited_from_system: false,
    },
    {
      allow_workspace_local_templates: true,
      allow_workspace_inherited_template_forks: true,
      allow_workspace_local_injectors: false,
    },
  );

  assert.equal(localBlocked.canEdit, false);
  assert.equal(localBlocked.canDelete, false);
  assert.deepEqual(localBlocked.badges, ["local", "readOnly"]);
});

test("system workspace owners can edit tenant default resources without read-only warnings", () => {
  const scope = {
    level: "workspace" as const,
    tenantCode: "acme",
    workspaceCode: "_system",
  };

  const templateState = getTemplateManagementState(
    scope,
    {
      owner_scope: "system",
      inherited_from_system: false,
      is_fork: false,
    },
    DEFAULT_WORKSPACE_POLICIES,
  );

  assert.equal(templateState.readOnly, false);
  assert.equal(templateState.canManageVersions, true);
  assert.deepEqual(templateState.badges, ["defaultSystem"]);

  const injectorState = getInjectorManagementState(
    scope,
    {
      owner_scope: "system",
      inherited_from_system: false,
    },
    DEFAULT_WORKSPACE_POLICIES,
  );

  assert.equal(injectorState.readOnly, false);
  assert.equal(injectorState.canEdit, true);
  assert.deepEqual(injectorState.badges, ["defaultSystem"]);
});

test("display scope resolves system-owned resources without relying on scope_level", () => {
  assert.equal(
    resolveResourceDisplayScope({
      owner_scope: "system",
      inherited_from_system: true,
      scope_level: undefined,
    }),
    "system",
  );

  assert.equal(
    resolveResourceDisplayScope({
      owner_scope: "local",
      workspace_id: "ws-123",
    }),
    "workspace",
  );

  assert.equal(
    resolveResourceDisplayScope({
      owner_scope: "global",
      scope_level: "global",
    }),
    "global",
  );
});

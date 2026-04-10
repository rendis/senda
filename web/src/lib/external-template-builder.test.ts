import test from "node:test";
import assert from "node:assert/strict";
import { resolveExternalTemplateBuilderViewState } from "./external-template-builder.ts";

const PROFILE_SLUG = "partner-embed";
const TENANT_CODE = "acme";
const WORKSPACE_CODE = "marketing";

test("embedded builder falls back to read-only on the system workspace", () => {
  assert.deepEqual(
    resolveExternalTemplateBuilderViewState({
      profileSlug: PROFILE_SLUG,
      scope: {
        level: "workspace",
        tenantCode: TENANT_CODE,
        workspaceCode: "_system",
      },
    }),
    {
      readOnly: true,
      readOnlyReason: "Resolved to the tenant Default workspace fallback.",
      workspaceLabel: "Default",
      scopeLabel: "Default scope",
      canEdit: false,
      canPublish: false,
      canTestSend: false,
      accessDenied: false,
    },
  );
});

test("embedded builder remains editable for a regular workspace", () => {
  assert.deepEqual(
    resolveExternalTemplateBuilderViewState({
      profileSlug: PROFILE_SLUG,
      scope: {
        level: "workspace",
        tenantCode: TENANT_CODE,
        workspaceCode: WORKSPACE_CODE,
      },
    }),
    {
      readOnly: false,
      workspaceLabel: WORKSPACE_CODE,
      scopeLabel: "Workspace",
      canEdit: true,
      canPublish: true,
      canTestSend: true,
      accessDenied: false,
    },
  );
});

test("embedded builder uses external permissions to disable mutating actions for partial viewers", () => {
  assert.deepEqual(
    resolveExternalTemplateBuilderViewState({
      profileSlug: PROFILE_SLUG,
      scope: {
        level: "workspace",
        tenantCode: TENANT_CODE,
        workspaceCode: WORKSPACE_CODE,
      },
      session: {
        read_only: false,
        effective_workspace_code: WORKSPACE_CODE,
        permissions: {
          list_templates: true,
          view_versions: true,
          edit_versions: false,
          publish_versions: false,
          test_send: false,
          builder_access: true,
          metadata_access: true,
          locale_access: true,
        },
      },
    }),
    {
      readOnly: false,
      workspaceLabel: WORKSPACE_CODE,
      scopeLabel: "Workspace",
      canEdit: false,
      canPublish: false,
      canTestSend: false,
      accessDenied: false,
    },
  );
});

test("embedded builder surfaces access denied when builder access is missing", () => {
  assert.deepEqual(
    resolveExternalTemplateBuilderViewState({
      profileSlug: PROFILE_SLUG,
      scope: {
        level: "workspace",
        tenantCode: TENANT_CODE,
        workspaceCode: WORKSPACE_CODE,
      },
      accessDenied: true,
    }),
    {
      readOnly: true,
      workspaceLabel: WORKSPACE_CODE,
      scopeLabel: "Workspace",
      canEdit: false,
      canPublish: false,
      canTestSend: false,
      accessDenied: true,
    },
  );
});

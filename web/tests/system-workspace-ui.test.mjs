import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { repoRoot } from "./test-root.mjs";

const root = repoRoot;

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("system sidebar exposes templates and injectors", () => {
  const source = read("web/src/components/layout/sidebar.tsx");

  assert.match(source, /templates[\s\S]*system:\s*true/, "Templates must be visible in system scope");
  assert.match(source, /injectors[\s\S]*system:\s*true/, "Injectors must be visible in system scope");
});

test("user-facing workspace UI uses display helpers instead of raw _system labels", () => {
  const helper = read("web/src/lib/system-workspace-display.ts");
  const switcher = read("web/src/components/shared/scope-switcher.tsx");
  const workspaces = read("web/src/components/workspaces/workspaces-content.tsx");
  const indicator = read("web/src/components/shared/scope-indicator.tsx");
  const dashboard = read("web/src/components/dashboard/dashboard-content.tsx");
  const members = read("web/src/components/members/members-content.tsx");

  assert.match(helper, /SYSTEM_WORKSPACE_LABEL = "Default"/, "System workspace presentation label must be Default");
  assert.match(switcher, /getWorkspaceDisplay(Name|Code)/, "Scope switcher must use workspace display helpers");
  assert.match(workspaces, /getWorkspaceDisplayName/, "Workspace table must use display name helper");
  assert.match(workspaces, /getWorkspaceDisplayCode/, "Workspace table must use display code helper");
  assert.match(
    workspaces,
    /aria-label=\{`Toggle workspace \$\{displayCode\} status`\}/,
    "Workspace toggle aria-label must use the display code helper",
  );
  assert.match(
    workspaces,
    /aria-label=\{`Edit workspace \$\{displayCode\}`\}/,
    "Workspace edit aria-label must use the display code helper",
  );
  assert.match(
    workspaces,
    /aria-label=\{`Delete workspace \$\{displayCode\}`\}/,
    "Workspace delete aria-label must use the display code helper",
  );
  assert.match(indicator, /labelKey: "system"/, "Scope indicator must map system scope through translated labels");
  assert.match(indicator, /t\(config\.labelKey\)/, "Scope indicator must render the translated label");
  assert.match(dashboard, /SYSTEM_WORKSPACE_(LABEL|SCOPE_LABEL)|getWorkspaceDisplay/, "Dashboard scope summary must use system workspace display helpers");
  assert.match(members, /SYSTEM_WORKSPACE_(LABEL|SCOPE_LABEL)|getWorkspaceDisplay/, "Members UI must not build labels from raw workspace code for the system workspace");
});

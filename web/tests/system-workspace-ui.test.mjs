import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("system sidebar exposes templates and injectors", () => {
  const source = read("web/src/components/layout/sidebar.tsx");

  assert.match(source, /templates[\s\S]*system:\s*true/, "Templates must be visible in system scope");
  assert.match(source, /injectors[\s\S]*system:\s*true/, "Injectors must be visible in system scope");
});

test("user-facing workspace UI uses display helpers instead of raw _system labels", () => {
  const switcher = read("web/src/components/shared/scope-switcher.tsx");
  const workspaces = read("web/src/components/workspaces/workspaces-content.tsx");
  const indicator = read("web/src/components/shared/scope-indicator.tsx");

  assert.match(switcher, /getWorkspaceDisplay(Name|Code)/, "Scope switcher must use workspace display helpers");
  assert.match(workspaces, /getWorkspaceDisplayName/, "Workspace table must use display name helper");
  assert.match(workspaces, /getWorkspaceDisplayCode/, "Workspace table must use display code helper");
  assert.doesNotMatch(workspaces, /aria-label={`Toggle workspace \$\{row\.original\.code\} status`}/, "Workspace toggle aria-label must not expose raw technical code");
  assert.doesNotMatch(workspaces, /aria-label={`Edit workspace \$\{row\.original\.code\}`}/, "Workspace edit aria-label must not expose raw technical code");
  assert.doesNotMatch(workspaces, /aria-label={`Delete workspace \$\{row\.original\.code\}`}/, "Workspace delete aria-label must not expose raw technical code");
  assert.match(indicator, /SYSTEM_WORKSPACE_LABEL/, "Scope indicator must not expose raw _system label");
});

import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

import { repoRoot } from "./test-root.mjs";

const cwd = repoRoot;
const root = cwd.endsWith("/web") ? cwd : path.join(cwd, "web");

async function read(relativePath) {
  const normalized =
    relativePath.startsWith("web/") && root.endsWith("/web")
      ? relativePath.slice(4)
      : relativePath;

  return readFile(path.join(root, normalized), "utf8");
}

test("settings page exposes _system workspace policy controls", async () => {
  const source = await read("web/src/components/settings/settings-content.tsx");
  const en = await read("web/messages/en.json");
  const hooks = await read("web/src/hooks/use-settings.ts");

  assert.match(source, /useTranslations\("settingsPage"\)/);
  assert.match(source, /onClick=\{\(\) => void savePolicies\(getValues\(\)\)\}/);
  assert.match(
    source,
    /useUpdateWorkspacePolicies\([\s\S]*scope\.workspaceCode[\s\S]*scope\.environment[\s\S]*\)/,
  );
  assert.match(source, /useResolvedWorkspacePolicies\(scope\)/);
  assert.match(hooks, /workspaceCode = "_system"/);
  assert.doesNotMatch(hooks, /query\.data \?\? DEFAULT_WORKSPACE_POLICIES/);
  assert.match(en, /"workspaceDefaultsPolicy"/);
});

test("template screens surface fork and read-only workspace copy", async () => {
  const list = await read("web/src/components/templates/templates-list-content.tsx");
  const types = await read("web/src/components/templates/template-types-content.tsx");
  const badges = await read("web/src/components/shared/resource-state-badges.tsx");

  assert.match(list, /useTranslations\("templatesPage"\)/);
  assert.match(types, /resolveResourceDisplayScope/);
  assert.match(badges, /useTranslations\("resourceState"\)/);
});

test("injector screens request inherited injectors and surface read-only copy", async () => {
  const source = await read("web/src/components/injectors/injectors-content.tsx");
  const scope = await read("web/src/components/shared/scope-indicator.tsx");

  assert.match(source, /includeInherited: scope\.level === "workspace"/);
  assert.match(source, /useTranslations\("injectorsPage"\)/);
  assert.match(scope, /useTranslations\("scopeIndicator"\)/);
});

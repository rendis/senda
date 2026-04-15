import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { repoRoot } from "./test-root.mjs";

const root = repoRoot;

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("system workspace display helpers keep internal code and expose Default presentation", () => {
  const source = read("web/src/lib/system-workspace-display.ts");

  assert.match(source, /SYSTEM_WORKSPACE_LABEL = "Default"/, "System workspace label must be Default");
  assert.match(source, /SYSTEM_WORKSPACE_SCOPE_LABEL = "Default scope"/, "System workspace scope label must be Default scope");
  assert.match(source, /from "@\/types\/api"/, "Helper must reuse SYSTEM_WORKSPACE_CODE from shared API types");
  assert.match(source, /code === SYSTEM_WORKSPACE_CODE/, "System workspace checks must use the shared code constant");
  assert.match(source, /`\/t\/\$\{tenantCode\}\/w\/\$\{SYSTEM_WORKSPACE_CODE\}\$\{suffix\}`/, "System workspace path helper must compose routes from the shared code constant");
});

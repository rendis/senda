import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { join } from "node:path";

import { repoRoot } from "./test-root.mjs";

const root = repoRoot;

test("tenant management frontend typechecks", () => {
  try {
    execFileSync("corepack", ["pnpm", "typecheck"], {
      cwd: join(root, "web"),
      stdio: "pipe",
    });
  } catch (error) {
    const stdout = error.stdout?.toString() ?? "";
    const stderr = error.stderr?.toString() ?? "";
    assert.fail(
      `Expected web TypeScript compilation to pass.\n${stdout}${stderr}`.trim(),
    );
  }
});

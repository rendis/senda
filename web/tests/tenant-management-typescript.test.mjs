import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";

const root = process.cwd();

test("tenant management frontend typechecks", () => {
  try {
    execFileSync("corepack", ["pnpm", "--dir", "web", "typecheck"], {
      cwd: root,
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

import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { join } from "node:path";

const root = process.cwd();

test("tenant management frontend typechecks", () => {
  try {
    execFileSync("npm", ["run", "typecheck"], {
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

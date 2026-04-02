import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("root layout does not import Fumadocs global stylesheet", () => {
  const rootLayout = read("web/src/app/layout.tsx");

  assert.equal(
    rootLayout.includes('import "fumadocs-ui/style.css";'),
    false,
    "Fumadocs stylesheet must not be imported in the root layout",
  );
});

test("global app stylesheet does not import Fumadocs CSS presets", () => {
  const globals = read("web/src/app/globals.css");

  assert.equal(
    globals.includes('@import "fumadocs-ui/css/neutral.css";'),
    false,
    "Fumadocs neutral.css must be isolated from the dashboard global stylesheet",
  );
  assert.equal(
    globals.includes('@import "fumadocs-ui/css/preset.css";'),
    false,
    "Fumadocs preset.css must be isolated from the dashboard global stylesheet",
  );
});

for (const layoutPath of [
  "web/src/app/(dashboard)/global/help/layout.tsx",
  "web/src/app/(dashboard)/t/[tenantCode]/help/layout.tsx",
]) {
  test(`${layoutPath} exists and imports Fumadocs CSS only inside help routes`, () => {
    assert.equal(existsSync(join(root, layoutPath)), true, `${layoutPath} must exist`);

    const layout = read(layoutPath);

    assert.equal(
      layout.includes('import "fumadocs-ui/style.css";'),
      true,
      `${layoutPath} must import the Fumadocs base stylesheet locally`,
    );
    assert.equal(
      layout.includes('import "fumadocs-ui/css/neutral.css";'),
      true,
      `${layoutPath} must import the Fumadocs neutral preset locally`,
    );
    assert.equal(
      layout.includes('import "fumadocs-ui/css/preset.css";'),
      true,
      `${layoutPath} must import the Fumadocs preset locally`,
    );
  });
}

test("no shared wrapper stylesheet is needed for help docs", () => {
  assert.equal(
    existsSync(join(root, "web/src/app/help-docs.css")),
    false,
    "help-docs.css should not exist because wrapping Fumadocs imports breaks CI",
  );
});

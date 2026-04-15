import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync, readdirSync } from "node:fs";
import { join } from "node:path";

import { repoRoot } from "./test-root.mjs";

const root = repoRoot;

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

function walk(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const absolutePath = join(dir, entry.name);
    if (entry.isDirectory()) return walk(absolutePath);
    return [absolutePath];
  });
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

test("no shared wrapper stylesheet is needed for help docs", () => {
  assert.equal(
    existsSync(join(root, "web/src/app/help-docs.css")),
    false,
    "help-docs.css should not exist because wrapping Fumadocs imports breaks CI",
  );
});

test("no App Router source file imports Fumadocs CSS directly", () => {
  const appFiles = walk(join(root, "web/src/app")).filter((file) =>
    /\.(css|tsx?|jsx?)$/.test(file),
  );

  for (const absolutePath of appFiles) {
    const contents = readFileSync(absolutePath, "utf8");

    assert.equal(
      contents.includes("fumadocs-ui/style.css"),
      false,
      `${absolutePath} must not import fumadocs-ui/style.css`,
    );
    assert.equal(
      contents.includes("fumadocs-ui/css/neutral.css"),
      false,
      `${absolutePath} must not import fumadocs-ui/css/neutral.css`,
    );
    assert.equal(
      contents.includes("fumadocs-ui/css/preset.css"),
      false,
      `${absolutePath} must not import fumadocs-ui/css/preset.css`,
    );
  }
});

import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("root layout does not preload IBM Plex Mono globally", () => {
  const layout = read("web/src/app/layout.tsx");
  const match = layout.match(/const ibmPlexMono = IBM_Plex_Mono\(([^]*?)\);/);

  assert.ok(match, "IBM Plex Mono font config must exist in root layout");
  assert.match(
    match[1],
    /preload:\s*false/,
    "IBM Plex Mono must opt out of global preload in the root layout",
  );
});

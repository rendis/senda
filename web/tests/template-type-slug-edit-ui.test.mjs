import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

import { repoRoot } from "./test-root.mjs";

const root = repoRoot;

test("template type edit dialog exposes slug reset and inline warning copy", async () => {
  const source = await readFile(
    path.join(root, "web/src/components/templates/template-types-content.tsx"),
    "utf8",
  );

  assert.match(source, /RotateCcw/, "Edit dialog should expose a reset icon inside the slug input");
  assert.match(source, /Restaurar slug original/, "Reset control should expose an accessible label");
  assert.match(source, /Cambiar el slug puede romper referencias existentes\./);
  assert.match(source, /Integraciones que usan el ref actual pueden dejar de resolver\./);
  assert.match(source, /Links o bookmarks al template type actual pueden quedar obsoletos\./);
  assert.match(source, /El historial y los filtros por slug pueden quedar divididos entre el valor viejo y el nuevo\./);
});

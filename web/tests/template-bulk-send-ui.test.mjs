import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

const root = process.cwd();

test("template editor wires a dedicated Bulk Send action and modal", async () => {
  const source = await readFile(
    path.join(root, "web/src/components/templates/mjml-editor.tsx"),
    "utf8"
  );

  assert.match(source, /BulkSendModal/);
  assert.match(source, /showBulkSend/);
  assert.match(source, /Bulk Send/);
  assert.match(source, /bulkSendEnabled = scope\.level === "workspace"/);
});

test("template version hooks expose bulk send endpoints", async () => {
  const source = await readFile(
    path.join(root, "web/src/hooks/use-template-version.ts"),
    "utf8"
  );

  assert.match(source, /useTemplateBulkSendConfig/);
  assert.match(source, /useTemplateBulkSend/);
  assert.match(source, /bulk-send-config/);
  assert.match(source, /bulk-send/);
});

test("bulk send modal documents items-only JSON, preview, and confirmation copy", async () => {
  const source = await readFile(
    path.join(root, "web/src/components/templates/bulk-send-modal.tsx"),
    "utf8"
  );

  assert.match(source, /published version/i);
  assert.match(source, /items\[\] only/i);
  assert.match(source, /Preview/);
  assert.match(source, /Confirm & Queue/);
  assert.match(source, /Send\s*Test/);
});

import test from "node:test";
import assert from "node:assert/strict";

const { getVersionPrimaryAction } = await import(
  new URL("./templates-list-actions.ts", import.meta.url).href,
);

test("published versions expose an explicit open action instead of view semantics", () => {
  assert.deepEqual(getVersionPrimaryAction("published"), {
    icon: "file-text",
    labelKey: "openVersion",
  });
});

test("draft versions keep edit semantics", () => {
  assert.deepEqual(getVersionPrimaryAction("draft"), {
    icon: "pencil",
    labelKey: "editVersion",
  });
});

test("draft versions can switch to open semantics when the template is read-only", () => {
  assert.deepEqual(getVersionPrimaryAction("draft", "open"), {
    icon: "file-text",
    labelKey: "openVersion",
  });
});

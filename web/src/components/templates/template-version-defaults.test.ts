import test from "node:test";
import assert from "node:assert/strict";

const { DEFAULT_MJML, buildDefaultEditorData } = await import(
  new URL("./template-version-defaults.ts", import.meta.url).href,
);

test("default MJML greeting uses a plain hello wave instead of token placeholder", () => {
  assert.match(DEFAULT_MJML, /Hello 👋/);
  assert.doesNotMatch(DEFAULT_MJML, /\{\{.*\}\}/);
});

test("default editor data starts with a text block using plain hello wave content", () => {
  const data = buildDefaultEditorData(() => "fixed-id");

  assert.deepEqual(data, {
    version: 1,
    blocks: [
      {
        id: "fixed-id",
        type: "text",
        content: "<p>Hello 👋</p>",
        align: "left",
      },
    ],
  });
});

import test from "node:test";
import assert from "node:assert/strict";

const { METADATA_TOKEN_INPUT_EDITOR_CLASSNAME } = await import(
  new URL("./metadata-token-input-styles.ts", import.meta.url).href
);

test("metadata token input wraps chip content instead of forcing horizontal scroll", () => {
  assert.match(METADATA_TOKEN_INPUT_EDITOR_CLASSNAME, /whitespace-pre-wrap/);
  assert.doesNotMatch(METADATA_TOKEN_INPUT_EDITOR_CLASSNAME, /overflow-x-auto/);
  assert.doesNotMatch(METADATA_TOKEN_INPUT_EDITOR_CLASSNAME, /whitespace-nowrap/);
});

import test from "node:test";
import assert from "node:assert/strict";

const { TEST_SEND_INJECTOR_HELPER_TEXT } = await import(
  new URL("./test-send-modal-copy.ts", import.meta.url).href
);

test("test send helper text explains that only overwriteable fields are shown", () => {
  assert.match(TEST_SEND_INJECTOR_HELPER_TEXT, /Only overwritable fields are shown/i);
  assert.match(TEST_SEND_INJECTOR_HELPER_TEXT, /static\/locked values resolve automatically/i);
});

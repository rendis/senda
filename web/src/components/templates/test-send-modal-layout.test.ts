import test from "node:test";
import assert from "node:assert/strict";

const { getTestSendScrollableBodyClassName } = await import(
  new URL("./test-send-modal-layout.ts", import.meta.url).href,
);

test("adds symmetric inline padding to the scrollable body so focused inputs are not clipped", () => {
  const className = getTestSendScrollableBodyClassName();

  assert.match(className, /\bpx-1\b/);
  assert.doesNotMatch(className, /\bpr-1\b/);
});

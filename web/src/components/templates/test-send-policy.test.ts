import test from "node:test";
import assert from "node:assert/strict";

const { getTestSendAvailability } = await import(
  new URL("./test-send-policy.ts", import.meta.url).href,
);

test("disables test send when template type has no adapter", () => {
  assert.deepEqual(getTestSendAvailability({ adapterId: undefined }), {
    enabled: false,
    reason: "Assign an adapter to this template type before sending test emails.",
  });
});

test("enables test send when template type has an adapter", () => {
  assert.deepEqual(getTestSendAvailability({ adapterId: "adapter-123" }), {
    enabled: true,
  });
});

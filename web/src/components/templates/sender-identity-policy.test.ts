import test from "node:test";
import assert from "node:assert/strict";

const {
  adapterUsesSenderIdentity,
  requiresExplicitSenderIdentity,
} = await import(new URL("./sender-identity-policy.ts", import.meta.url).href);

test("ses and smtp adapters use sender identities", () => {
  assert.equal(adapterUsesSenderIdentity({ adapter_type: "ses" }), true);
  assert.equal(adapterUsesSenderIdentity({ adapter_type: "smtp" }), true);
  assert.equal(adapterUsesSenderIdentity({ adapter_type: "gmail" }), false);
  assert.equal(adapterUsesSenderIdentity(undefined), false);
});

test("shared ses and smtp adapters require explicit sender identity", () => {
  assert.equal(
    requiresExplicitSenderIdentity({ adapter_type: "ses", is_shared: true }),
    true,
  );
  assert.equal(
    requiresExplicitSenderIdentity({ adapter_type: "smtp", is_shared: true }),
    true,
  );
  assert.equal(
    requiresExplicitSenderIdentity({ adapter_type: "smtp", is_shared: false }),
    false,
  );
  assert.equal(
    requiresExplicitSenderIdentity({ adapter_type: "gmail", is_shared: true }),
    false,
  );
});

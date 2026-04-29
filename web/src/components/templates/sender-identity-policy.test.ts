import test from "node:test";
import assert from "node:assert/strict";

const {
  adapterUsesSenderIdentity,
  requiresExplicitSenderIdentity,
  resolveTemplateTypeSenderIdentityId,
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

test("template type update serializes cleared sender identity as an explicit empty string", () => {
  assert.equal(
    resolveTemplateTypeSenderIdentityId("", { clearWithEmptyString: true }),
    "",
  );
  assert.equal(
    resolveTemplateTypeSenderIdentityId("__default__", { clearWithEmptyString: true }),
    "",
  );
});

test("template type create omits blank/default sender identity", () => {
  assert.equal(resolveTemplateTypeSenderIdentityId(""), undefined);
  assert.equal(resolveTemplateTypeSenderIdentityId("__default__"), undefined);
  assert.equal(resolveTemplateTypeSenderIdentityId(" sender-id "), "sender-id");
});

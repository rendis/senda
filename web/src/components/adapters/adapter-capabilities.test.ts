import test from "node:test";
import assert from "node:assert/strict";

const {
  canTestSendAdapter,
  getAdapterCapabilities,
  shouldIncludeTestSendFrom,
} = await import(new URL("./adapter-capabilities.ts", import.meta.url).href);

test("smtp supports sender identities without provider sync or provisioning", () => {
  assert.deepEqual(getAdapterCapabilities("smtp"), {
    supportsIdentitySync: false,
    supportsProvisioning: false,
    supportsSenderSharing: true,
    supportsAdapterSharing: false,
    usesSenderIdentity: true,
  });
});

test("provider adapters keep their existing capability split", () => {
  assert.equal(getAdapterCapabilities("ses").supportsIdentitySync, true);
  assert.equal(getAdapterCapabilities("ses").supportsProvisioning, true);
  assert.equal(getAdapterCapabilities("ses").supportsSenderSharing, true);
  assert.equal(getAdapterCapabilities("gmail").supportsIdentitySync, true);
  assert.equal(getAdapterCapabilities("gmail").supportsAdapterSharing, true);
  assert.equal(getAdapterCapabilities("gmail").supportsSenderSharing, false);
});

test("smtp and ses test sends require a verified email sender", () => {
  assert.equal(canTestSendAdapter("smtp", []), false);
  assert.equal(canTestSendAdapter("ses", []), false);
  assert.equal(canTestSendAdapter("gmail", []), true);
  assert.equal(
    canTestSendAdapter("smtp", [
      { identity_type: "email", status: "verified" },
    ]),
    true,
  );
});

test("test send includes from for smtp and ses only", () => {
  assert.equal(shouldIncludeTestSendFrom("smtp", "sender@example.com"), true);
  assert.equal(shouldIncludeTestSendFrom("ses", "sender@example.com"), true);
  assert.equal(shouldIncludeTestSendFrom("gmail", "sender@example.com"), false);
  assert.equal(shouldIncludeTestSendFrom("smtp", ""), false);
});

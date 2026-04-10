import assert from "node:assert/strict";
import test from "node:test";

import {
  formatWorkspaceTestRecipientAddresses,
  normalizeWorkspaceTestRecipientAddresses,
} from "./workspace-test-recipient-addresses.ts";

test("normalizeWorkspaceTestRecipientAddresses tolerates nullish and malformed values", () => {
  assert.deepEqual(normalizeWorkspaceTestRecipientAddresses(undefined), []);
  assert.deepEqual(normalizeWorkspaceTestRecipientAddresses(null), []);
  assert.deepEqual(
    normalizeWorkspaceTestRecipientAddresses([
      "qa@example.com",
      42 as unknown as string,
      "ops@example.com",
    ]),
    ["qa@example.com", "ops@example.com"],
  );
});

test("formatWorkspaceTestRecipientAddresses returns a multiline string for the form", () => {
  assert.equal(formatWorkspaceTestRecipientAddresses(undefined), "");
  assert.equal(formatWorkspaceTestRecipientAddresses(null), "");
  assert.equal(
    formatWorkspaceTestRecipientAddresses(["qa@example.com", "ops@example.com"]),
    "qa@example.com\nops@example.com",
  );
});

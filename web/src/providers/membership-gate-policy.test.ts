import assert from "node:assert/strict";
import test from "node:test";

import {
  getMembershipGateSessionKey,
  shouldShowMembershipGateLoader,
} from "./membership-gate-policy.ts";

test("membership gate key is stable across dashboard pathname changes", () => {
  const idToken = "token-123";

  assert.equal(getMembershipGateSessionKey("authenticated", idToken), idToken);
  assert.equal(
    shouldShowMembershipGateLoader({
      status: "authenticated",
      sessionKey: idToken,
      checkedSessionKey: idToken,
    }),
    false,
  );
});

test("membership gate blocks while a new session has not been checked", () => {
  assert.equal(
    shouldShowMembershipGateLoader({
      status: "authenticated",
      sessionKey: "new-token",
      checkedSessionKey: "old-token",
    }),
    true,
  );
});

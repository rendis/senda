import test from "node:test";
import assert from "node:assert/strict";

const { shouldTriggerFederatedLogout } = await import(
  new URL("./session-guard-policy.ts", import.meta.url).href,
);

test("triggers logout on authenticated refresh token error in app routes", () => {
  assert.equal(
    shouldTriggerFederatedLogout({
      pathname: "/global",
      status: "authenticated",
      sessionError: "RefreshTokenError",
      alreadyTriggered: false,
    }),
    true,
  );
});

test("does not retrigger logout while already on /logout", () => {
  assert.equal(
    shouldTriggerFederatedLogout({
      pathname: "/logout",
      status: "authenticated",
      sessionError: "RefreshTokenError",
      alreadyTriggered: false,
    }),
    false,
  );
});

test("does not trigger federated logout on /login", () => {
  assert.equal(
    shouldTriggerFederatedLogout({
      pathname: "/login",
      status: "authenticated",
      sessionError: "RefreshTokenError",
      alreadyTriggered: false,
    }),
    false,
  );
});

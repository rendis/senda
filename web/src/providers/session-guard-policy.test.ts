import test from "node:test";
import assert from "node:assert/strict";

const { isExternalEmbedPath, shouldRedirectUnauthenticatedToLogin, shouldTriggerFederatedLogout } = await import(
  new URL("./session-guard-policy.ts", import.meta.url).href,
);

test("detects embed routes for external session mode", () => {
  assert.equal(
    isExternalEmbedPath("/embed/partner-portal/t/acme/w/main/templates/newsletter/edit"),
    true,
  );
  assert.equal(isExternalEmbedPath("/global"), false);
});

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

test("does not trigger federated logout on embed routes", () => {
  assert.equal(
    shouldTriggerFederatedLogout({
      pathname: "/embed/partner-portal/t/acme/w/main/templates/newsletter/edit",
      status: "authenticated",
      sessionError: "RefreshTokenError",
      alreadyTriggered: false,
    }),
    false,
  );
});

test("redirects unauthenticated users on app routes", () => {
  assert.equal(
    shouldRedirectUnauthenticatedToLogin({
      pathname: "/global",
      status: "unauthenticated",
      alreadyTriggered: false,
    }),
    true,
  );
});

test("does not redirect unauthenticated users on embed routes", () => {
  assert.equal(
    shouldRedirectUnauthenticatedToLogin({
      pathname: "/embed/partner-portal/t/acme/w/main/templates/newsletter/edit",
      status: "unauthenticated",
      alreadyTriggered: false,
    }),
    false,
  );
});

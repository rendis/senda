import test from "node:test";
import assert from "node:assert/strict";

const { isExternalEmbedPath, isPublicAppPath, shouldRedirectUnauthenticatedToLogin, shouldTriggerFederatedLogout } = await import(
  new URL("./session-guard-policy.ts", import.meta.url).href,
);
const UNAUTHENTICATED_STATUS = "unauthenticated" as const;
const EMBED_PATH =
  "/embed/partner-portal/t/acme/w/main/templates/newsletter/edit";

test("detects embed routes for external session mode", () => {
  assert.equal(
    isExternalEmbedPath(EMBED_PATH),
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
      pathname: EMBED_PATH,
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
      status: UNAUTHENTICATED_STATUS,
      alreadyTriggered: false,
    }),
    true,
  );

  assert.equal(
    shouldRedirectUnauthenticatedToLogin({
      pathname: "/t/acme/w/main",
      status: UNAUTHENTICATED_STATUS,
      alreadyTriggered: false,
    }),
    true,
  );

  assert.equal(
    shouldRedirectUnauthenticatedToLogin({
      pathname: "/onboarding",
      status: UNAUTHENTICATED_STATUS,
      alreadyTriggered: false,
    }),
    true,
  );

  assert.equal(
    shouldRedirectUnauthenticatedToLogin({
      pathname: "/",
      status: UNAUTHENTICATED_STATUS,
      alreadyTriggered: false,
    }),
    true,
  );
});

test("does not redirect unauthenticated users on embed routes", () => {
  assert.equal(
    shouldRedirectUnauthenticatedToLogin({
      pathname: EMBED_PATH,
      status: UNAUTHENTICATED_STATUS,
      alreadyTriggered: false,
    }),
    false,
  );
});

test("does not redirect unauthenticated users on public routes", () => {
  const publicRoutes = ["/login", "/logout", "/access-denied"];

  for (const pathname of publicRoutes) {
    assert.equal(
      shouldRedirectUnauthenticatedToLogin({
        pathname,
        status: UNAUTHENTICATED_STATUS,
        alreadyTriggered: false,
      }),
      false,
      `expected ${pathname} to stay public`,
    );
  }
});

test("identifies explicitly public app routes", () => {
  assert.equal(isPublicAppPath("/login"), true);
  assert.equal(isPublicAppPath("/logout"), true);
  assert.equal(isPublicAppPath("/access-denied"), true);
  assert.equal(isPublicAppPath(EMBED_PATH), true);
  assert.equal(isPublicAppPath("/onboarding"), false);
  assert.equal(isPublicAppPath("/global"), false);
});

import test from "node:test";
import assert from "node:assert/strict";

const { decideOnboardingPageRedirect } = await import(
  new URL("./page-guard.ts", import.meta.url).href
);

test("redirects to login when there is no id token", () => {
  assert.equal(decideOnboardingPageRedirect(null, null), "/login");
});

test("redirects to global when onboarding is already completed", () => {
  assert.equal(
    decideOnboardingPageRedirect(
      { idToken: "token" },
      { ok: true, status: 200, data: { needs_onboarding: false } },
    ),
    "/global",
  );
});

test("redirects to login when onboarding status returns 401 for stale token", () => {
  assert.equal(
    decideOnboardingPageRedirect(
      { idToken: "stale-token" },
      { ok: false, status: 401 },
    ),
    "/login",
  );
});

test("redirects to login when onboarding status returns 403", () => {
  assert.equal(
    decideOnboardingPageRedirect(
      { idToken: "forbidden-token" },
      { ok: false, status: 403 },
    ),
    "/login",
  );
});

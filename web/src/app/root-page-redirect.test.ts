import test from "node:test";
import assert from "node:assert/strict";

const { decideRootRedirectPath } = await import(
  new URL("./root-page-redirect.ts", import.meta.url).href,
);

test("redirects unauthenticated visitors to login", () => {
  assert.equal(decideRootRedirectPath(null), "/login");
  assert.equal(decideRootRedirectPath(undefined), "/login");
  assert.equal(decideRootRedirectPath({}), "/login");
});

test("redirects authenticated visitors to the global dashboard", () => {
  assert.equal(decideRootRedirectPath({ idToken: "token" }), "/global");
});

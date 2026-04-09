import test from "node:test";
import assert from "node:assert/strict";

const { buildFederatedLogoutUrl } = await import(
  new URL("./logout-url.ts", import.meta.url).href
);

test("returns callback url when issuer is missing", () => {
  const callbackUrl = new URL("http://localhost:3000/login");
  assert.equal(
    buildFederatedLogoutUrl({
      issuer: undefined,
      clientId: "web",
      callbackUrl,
      session: null,
    }),
    callbackUrl.toString(),
  );
});

test("includes id_token_hint for healthy sessions", () => {
  const url = new URL(
    buildFederatedLogoutUrl({
      issuer: "http://localhost:55123/realms/senda",
      clientId: "web",
      callbackUrl: new URL("http://localhost:3000/login"),
      session: { idToken: "good-id-token" },
    }),
  );

  assert.equal(url.searchParams.get("client_id"), "web");
  assert.equal(url.searchParams.get("id_token_hint"), "good-id-token");
});

test("omits id_token_hint when session carries RefreshTokenError", () => {
  const url = new URL(
    buildFederatedLogoutUrl({
      issuer: "http://localhost:55123/realms/senda",
      clientId: "web",
      callbackUrl: new URL("http://localhost:3000/login"),
      session: { idToken: "stale-id-token", error: "RefreshTokenError" },
    }),
  );

  assert.equal(url.searchParams.get("client_id"), "web");
  assert.equal(url.searchParams.get("id_token_hint"), null);
});

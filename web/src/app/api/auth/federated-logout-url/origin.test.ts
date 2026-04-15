import assert from "node:assert/strict";
import test from "node:test";
import { NextRequest } from "next/server.js";

const { resolvePublicOrigin, sanitizeCallbackUrl } = await import(
  new URL("./origin.ts", import.meta.url).href
);

function makeRequest(url: string, host = "poisoned.example.com") {
  return new NextRequest(url, {
    headers: {
      host,
    },
  });
}

test("resolvePublicOrigin uses configured AUTH_URL when present", () => {
  process.env.AUTH_URL = "https://app.example.com";
  const request = makeRequest("https://internal.example.net/api/auth/federated-logout-url");

  assert.equal(resolvePublicOrigin(request), "https://app.example.com");
});

test("resolvePublicOrigin falls back to request origin instead of trusting Host", () => {
  delete process.env.AUTH_URL;
  const request = makeRequest(
    "https://app.example.com/api/auth/federated-logout-url",
    "evil.example.net",
  );

  assert.equal(resolvePublicOrigin(request), "https://app.example.com");
});

test("sanitizeCallbackUrl keeps same-origin callback paths", () => {
  const request = makeRequest("https://app.example.com/api/auth/federated-logout-url");

  const callbackUrl = sanitizeCallbackUrl(
    request,
    "https://app.example.com/logout-complete?from=oidc#done",
  );

  assert.equal(
    callbackUrl.toString(),
    "https://app.example.com/logout-complete?from=oidc#done",
  );
});

test("sanitizeCallbackUrl collapses cross-origin callbacks to local login", () => {
  const request = makeRequest("https://app.example.com/api/auth/federated-logout-url");

  const callbackUrl = sanitizeCallbackUrl(
    request,
    "https://evil.example.net/logout-complete",
  );

  assert.equal(callbackUrl.toString(), "https://app.example.com/login");
});

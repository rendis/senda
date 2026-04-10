import assert from "node:assert/strict";
import test from "node:test";
import { NextRequest, NextResponse } from "next/server.js";

import {
  applyExternalEmbedSecurityHeaders,
  buildExternalBootstrapUrl,
  createExternalEmbedProxyRouter,
  loadExternalEmbedFrameAncestors,
  normalizeFrameAncestors,
  type ProxyRouteContext,
} from "./external-embed-proxy.ts";

const EMBED_PATH = "/embed/partner-portal/t/acme/w/marketing/templates/newsletter/edit";
const BACKEND_ORIGIN = "https://backend.example.com";
const HOST_ORIGIN = "https://host.example.com";
const SELF_ANCESTOR = "'self'";
const NONE_ANCESTOR = "'none'";
const BOOTSTRAP_URL =
  "https://backend.example.com/api/v1/external/partner-portal/bootstrap";
const ROUTE_CONTEXT: ProxyRouteContext = {
  params: Promise.resolve({}),
};

function mockEmbedRequest(pathname: string) {
  return new NextRequest(`https://senda.example.com${pathname}`);
}

test("buildExternalBootstrapUrl targets the profile bootstrap endpoint", () => {
  const request = mockEmbedRequest(EMBED_PATH);

  const url = buildExternalBootstrapUrl(request, BACKEND_ORIGIN);

  assert.equal(
    url?.toString(),
    BOOTSTRAP_URL,
  );
});

test("normalizeFrameAncestors falls back to none when empty", () => {
  assert.deepEqual(normalizeFrameAncestors([]), [NONE_ANCESTOR]);
  assert.deepEqual(normalizeFrameAncestors([SELF_ANCESTOR, HOST_ORIGIN]), [
    SELF_ANCESTOR,
    HOST_ORIGIN,
  ]);
});

test("loadExternalEmbedFrameAncestors falls back to none on bootstrap errors", async () => {
  const frameAncestors = await loadExternalEmbedFrameAncestors(
    mockEmbedRequest(EMBED_PATH),
    BACKEND_ORIGIN,
    async () =>
      new Response("nope", {
        status: 500,
      }),
  );

  assert.deepEqual(frameAncestors, [NONE_ANCESTOR]);
});

test("loadExternalEmbedFrameAncestors returns profile frame ancestors from bootstrap", async () => {
  const frameAncestors = await loadExternalEmbedFrameAncestors(
    mockEmbedRequest(EMBED_PATH),
    BACKEND_ORIGIN,
    async (input) => {
      assert.equal(
        String(input),
        BOOTSTRAP_URL,
      );
      return new Response(
        JSON.stringify({
          frame_ancestors: [SELF_ANCESTOR, HOST_ORIGIN],
        }),
        {
          status: 200,
          headers: {
            "content-type": "application/json",
          },
        },
      );
    },
  );

  assert.deepEqual(frameAncestors, [SELF_ANCESTOR, HOST_ORIGIN]);
});

test("applyExternalEmbedSecurityHeaders sets frame-ancestors and removes x-frame-options", () => {
  const response = NextResponse.next();
  response.headers.set("X-Frame-Options", "DENY");

  applyExternalEmbedSecurityHeaders(response, [SELF_ANCESTOR, HOST_ORIGIN]);

  assert.equal(
    response.headers.get("Content-Security-Policy"),
    `frame-ancestors ${SELF_ANCESTOR} ${HOST_ORIGIN}`,
  );
  assert.equal(response.headers.get("X-Frame-Options"), null);
});

test("createExternalEmbedProxyRouter bypasses auth for embed requests", async () => {
  let authenticatedProxyCalls = 0;
  const proxy = createExternalEmbedProxyRouter({
    authenticatedProxy: async () => {
      authenticatedProxyCalls += 1;
      return NextResponse.next();
    },
    externalEmbedProxyRequest: async () =>
      NextResponse.next({
        headers: {
          "Content-Security-Policy": `frame-ancestors ${NONE_ANCESTOR}`,
        },
      }),
  });

  const response = (await proxy(mockEmbedRequest(EMBED_PATH), ROUTE_CONTEXT)) as Response;

  assert.equal(authenticatedProxyCalls, 0);
  assert.equal(
    response.headers.get("Content-Security-Policy"),
    `frame-ancestors ${NONE_ANCESTOR}`,
  );
});

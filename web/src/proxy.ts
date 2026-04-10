import { NextRequest, NextResponse } from "next/server.js";
import { auth } from "@/auth";
import { LOCALE_COOKIE, DEFAULT_LOCALE } from "@/lib/locale";
import {
  applyExternalEmbedSecurityHeaders,
  createExternalEmbedProxyRouter,
  loadExternalEmbedFrameAncestors,
} from "@/lib/external-embed-proxy";

const defaultBackendOrigin =
  process.env.SENDA_SERVER_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  "http://localhost:8080";

function buildBaseProxyResponse(req: NextRequest): NextResponse {
  const requestHeaders = new Headers(req.headers);
  requestHeaders.set("x-senda-pathname", req.nextUrl.pathname);

  const res = NextResponse.next({
    request: {
      headers: requestHeaders,
    },
  });

  // Prevent browser bfcache from showing stale authenticated pages after logout.
  res.headers.set("Cache-Control", "no-store, no-cache, must-revalidate");
  res.headers.set("Pragma", "no-cache");

  if (!req.cookies.get(LOCALE_COOKIE)) {
    res.cookies.set(LOCALE_COOKIE, DEFAULT_LOCALE, {
      path: "/",
      maxAge: 60 * 60 * 24 * 365,
      sameSite: "lax",
    });
  }

  return res;
}

async function proxyExternalEmbedRequest(req: NextRequest): Promise<NextResponse> {
  const response = buildBaseProxyResponse(req);
  const frameAncestors = await loadExternalEmbedFrameAncestors(
    req,
    defaultBackendOrigin,
  );
  return applyExternalEmbedSecurityHeaders(response, frameAncestors);
}

const authenticatedProxy = auth(function proxy(req: NextRequest) {
  return buildBaseProxyResponse(req);
});

export default createExternalEmbedProxyRouter({
  authenticatedProxy,
  externalEmbedProxyRequest: proxyExternalEmbedRequest,
});

export const config = {
  matcher: [
    "/((?!login|access-denied|onboarding|api/|_next/static|_next/image|favicon.ico|senda-logo.svg).*)",
  ],
};

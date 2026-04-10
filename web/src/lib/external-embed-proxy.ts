import { NextRequest, NextResponse } from "next/server.js";
import { isExternalEmbedPath } from "./external-api-context.ts";

export type FetchLike = typeof fetch;
export interface ProxyRouteContext {
  params: Promise<Record<string, string | string[] | undefined>>;
}

export type ProxyHandler = (
  req: NextRequest,
  ctx: ProxyRouteContext,
) => Response | void | Promise<Response | void>;

interface ExternalBootstrapResponseShape {
  frame_ancestors?: string[];
}

export function buildExternalBootstrapUrl(
  request: NextRequest,
  backendOrigin: string,
): URL | null {
  const segments = request.nextUrl.pathname.split("/").filter(Boolean);
  if (segments[0] !== "embed" || !segments[1]) {
    return null;
  }

  return new URL(`/api/v1/external/${segments[1]}/bootstrap`, backendOrigin);
}

export function normalizeFrameAncestors(
  frameAncestors?: readonly string[],
): string[] {
  const cleaned = (frameAncestors ?? [])
    .map((value) => value.trim())
    .filter(Boolean);

  return cleaned.length > 0 ? cleaned : ["'none'"];
}

export function applyExternalEmbedSecurityHeaders(
  response: NextResponse,
  frameAncestors: readonly string[],
): NextResponse {
  response.headers.delete("X-Frame-Options");
  response.headers.set(
    "Content-Security-Policy",
    `frame-ancestors ${normalizeFrameAncestors(frameAncestors).join(" ")}`,
  );
  return response;
}

export async function loadExternalEmbedFrameAncestors(
  request: NextRequest,
  backendOrigin: string,
  fetchImpl: FetchLike = fetch,
): Promise<string[]> {
  const bootstrapUrl = buildExternalBootstrapUrl(request, backendOrigin);
  if (!bootstrapUrl) {
    return ["'none'"];
  }

  try {
    const upstream = await fetchImpl(bootstrapUrl, {
      method: "GET",
      headers: {
        accept: "application/json",
      },
      cache: "no-store",
    });

    if (!upstream.ok) {
      return ["'none'"];
    }

    const body = (await upstream.json()) as ExternalBootstrapResponseShape;
    return normalizeFrameAncestors(body.frame_ancestors);
  } catch {
    return ["'none'"];
  }
}

export function createExternalEmbedProxyRouter({
  authenticatedProxy,
  externalEmbedProxyRequest,
}: {
  authenticatedProxy: ProxyHandler;
  externalEmbedProxyRequest: ProxyHandler;
}) {
  return async function proxy(
    req: NextRequest,
    ctx: ProxyRouteContext,
  ): Promise<Response | void> {
    if (isExternalEmbedPath(req.nextUrl.pathname)) {
      return externalEmbedProxyRequest(req, ctx);
    }

    return authenticatedProxy(req, ctx);
  };
}

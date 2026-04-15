import { NextRequest } from "next/server.js";

export function resolvePublicOrigin(request: NextRequest): string {
  return process.env.AUTH_URL || request.nextUrl.origin;
}

export function sanitizeCallbackUrl(
  request: NextRequest,
  rawCallbackUrl: string | null,
): URL {
  const origin = resolvePublicOrigin(request);
  try {
    const candidate = new URL(rawCallbackUrl ?? "/login", origin);
    if (candidate.origin !== new URL(origin).origin) {
      return new URL("/login", origin);
    }

    return new URL(
      `${candidate.pathname}${candidate.search}${candidate.hash}`,
      origin,
    );
  } catch {
    return new URL("/login", origin);
  }
}

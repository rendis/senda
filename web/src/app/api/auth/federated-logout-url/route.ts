import { NextRequest, NextResponse } from "next/server";
import { auth } from "@/auth";
import { buildFederatedLogoutUrl } from "./logout-url";

function getPublicOrigin(request: NextRequest): string {
  return process.env.AUTH_URL || `https://${request.headers.get("host")}` || request.nextUrl.origin;
}

function sanitizeCallbackUrl(
  request: NextRequest,
  rawCallbackUrl: string | null,
): URL {
  const origin = getPublicOrigin(request);
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

export async function GET(request: NextRequest) {
  const session = await auth();
  const callbackUrl = sanitizeCallbackUrl(
    request,
    request.nextUrl.searchParams.get("callbackUrl"),
  );
  const url = buildFederatedLogoutUrl({
    issuer: process.env.AUTH_OIDC_ISSUER,
    clientId: process.env.AUTH_OIDC_ID,
    callbackUrl,
    session,
  });

  return NextResponse.json({ url });
}

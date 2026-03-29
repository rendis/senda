import { NextRequest, NextResponse } from "next/server";
import { auth } from "@/auth";

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
  const issuer = process.env.AUTH_OIDC_ISSUER;

  if (!issuer) {
    return NextResponse.json({ url: callbackUrl.toString() });
  }

  const logoutUrl = new URL(
    `${issuer.replace(/\/$/, "")}/protocol/openid-connect/logout`,
  );
  logoutUrl.searchParams.set(
    "post_logout_redirect_uri",
    callbackUrl.toString(),
  );

  if (process.env.AUTH_OIDC_ID) {
    logoutUrl.searchParams.set("client_id", process.env.AUTH_OIDC_ID);
  }

  if (session?.idToken) {
    logoutUrl.searchParams.set("id_token_hint", session.idToken);
  }

  return NextResponse.json({ url: logoutUrl.toString() });
}

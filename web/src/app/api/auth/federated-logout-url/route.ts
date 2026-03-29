import { NextRequest, NextResponse } from "next/server";
import { auth } from "@/auth";

const basePath = process.env.__NEXT_ROUTER_BASEPATH || "";

function sanitizeCallbackUrl(
  request: NextRequest,
  rawCallbackUrl: string | null,
): URL {
  const loginPath = `${basePath}/login`;
  try {
    const candidate = new URL(rawCallbackUrl ?? loginPath, request.nextUrl.origin);
    if (candidate.origin !== request.nextUrl.origin) {
      return new URL(loginPath, request.nextUrl.origin);
    }

    return new URL(
      `${candidate.pathname}${candidate.search}${candidate.hash}`,
      request.nextUrl.origin,
    );
  } catch {
    return new URL(loginPath, request.nextUrl.origin);
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

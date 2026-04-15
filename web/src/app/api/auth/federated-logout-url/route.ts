import { NextRequest, NextResponse } from "next/server.js";
import { auth } from "@/auth";
import { buildFederatedLogoutUrl } from "./logout-url";
import { sanitizeCallbackUrl } from "./origin";

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

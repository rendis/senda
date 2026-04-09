import type { Session } from "next-auth";

export interface LogoutUrlOptions {
  issuer: string | undefined;
  clientId: string | undefined;
  callbackUrl: URL;
  session: Pick<Session, "idToken" | "error"> | null;
}

export function buildFederatedLogoutUrl({
  issuer,
  clientId,
  callbackUrl,
  session,
}: LogoutUrlOptions): string {
  if (!issuer) {
    return callbackUrl.toString();
  }

  const logoutUrl = new URL(
    `${issuer.replace(/\/$/, "")}/protocol/openid-connect/logout`,
  );

  logoutUrl.searchParams.set(
    "post_logout_redirect_uri",
    callbackUrl.toString(),
  );

  if (clientId) {
    logoutUrl.searchParams.set("client_id", clientId);
  }

  const idToken = session?.idToken;
  const shouldSendIdTokenHint =
    !!idToken && session.error !== "RefreshTokenError";

  if (shouldSendIdTokenHint) {
    logoutUrl.searchParams.set("id_token_hint", idToken);
  }

  return logoutUrl.toString();
}

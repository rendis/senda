export function isExternalEmbedPath(pathname: string): boolean {
  return pathname.startsWith("/embed/");
}

export function shouldTriggerFederatedLogout(params: {
  pathname: string;
  status: "authenticated" | "unauthenticated" | "loading";
  sessionError?: "RefreshTokenError";
  alreadyTriggered: boolean;
}): boolean {
  const { pathname, status, sessionError, alreadyTriggered } = params;

  if (alreadyTriggered) return false;
  if (status !== "authenticated") return false;
  if (sessionError !== "RefreshTokenError") return false;
  if (isExternalEmbedPath(pathname)) return false;
  if (pathname === "/logout" || pathname === "/login") return false;

  return true;
}

export function shouldRedirectUnauthenticatedToLogin(params: {
  pathname: string;
  status: "authenticated" | "unauthenticated" | "loading";
  alreadyTriggered: boolean;
}): boolean {
  const { pathname, status, alreadyTriggered } = params;

  if (alreadyTriggered) return false;
  if (status !== "unauthenticated") return false;
  if (isExternalEmbedPath(pathname)) return false;

  return true;
}

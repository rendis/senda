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
  if (pathname === "/logout" || pathname === "/login") return false;

  return true;
}

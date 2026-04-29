type SessionStatus = "authenticated" | "loading" | "unauthenticated";

interface LoaderState {
  status: SessionStatus;
  sessionKey: string | null;
  checkedSessionKey: string | null;
}

export function getMembershipGateSessionKey(
  status: SessionStatus,
  idToken?: string | null,
): string | null {
  return status === "authenticated" && idToken ? idToken : null;
}

export function shouldShowMembershipGateLoader({
  status,
  sessionKey,
  checkedSessionKey,
}: LoaderState): boolean {
  return (
    status === "loading" ||
    status === "unauthenticated" ||
    !sessionKey ||
    checkedSessionKey !== sessionKey
  );
}

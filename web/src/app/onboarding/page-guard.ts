import type { Session } from "next-auth";

interface OnboardingStatusResponse {
  needs_onboarding: boolean;
}

export type OnboardingPageDecision = "/login" | "/global" | null;

export function decideOnboardingPageRedirect(
  session: Pick<Session, "idToken"> | null | undefined,
  statusResponse:
    | { ok: boolean; status: number; data?: OnboardingStatusResponse }
    | null,
): OnboardingPageDecision {
  if (!session?.idToken) {
    return "/login";
  }

  if (
    statusResponse &&
    (statusResponse.status === 401 || statusResponse.status === 403)
  ) {
    return "/login";
  }

  if (
    statusResponse?.ok &&
    statusResponse.data &&
    !statusResponse.data.needs_onboarding
  ) {
    return "/global";
  }

  return null;
}

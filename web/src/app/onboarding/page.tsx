import { redirect } from "next/navigation";
import { authWithoutRefresh } from "@/auth";
import { OnboardingWizard } from "./_components/wizard";
import type { OnboardingStatus } from "@/types/api";
import { decideOnboardingPageRedirect } from "./page-guard";

export default async function OnboardingPage() {
  const session = await authWithoutRefresh();
  const headers = session?.idToken
    ? { Authorization: `Bearer ${session.idToken}` }
    : undefined;

  const missingSessionRedirect = decideOnboardingPageRedirect(session, null);
  if (missingSessionRedirect) {
    redirect(missingSessionRedirect);
  }

  // Server-side onboarding check — redirect if already completed
  try {
    const apiUrl =
      process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
    const res = await fetch(`${apiUrl}/api/v1/onboarding/status`, {
      cache: "no-store",
      headers,
    });
    const data = res.ok ? ((await res.json()) as OnboardingStatus) : undefined;
    const statusRedirect = decideOnboardingPageRedirect(session, {
      ok: res.ok,
      status: res.status,
      data,
    });
    if (statusRedirect) {
      redirect(statusRedirect);
    }
  } catch {
    // Backend unavailable — show wizard anyway
  }

  return <OnboardingWizard />;
}

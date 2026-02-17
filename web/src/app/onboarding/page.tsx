import { redirect } from "next/navigation";
import { OnboardingWizard } from "./_components/wizard";
import type { OnboardingStatus } from "@/types/api";

export default async function OnboardingPage() {
  // Server-side onboarding check — redirect if already completed
  try {
    const apiUrl =
      process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
    const res = await fetch(`${apiUrl}/api/v1/onboarding/status`, {
      cache: "no-store",
    });
    if (res.ok) {
      const data: OnboardingStatus = await res.json();
      if (!data.needs_onboarding) {
        redirect("/global");
      }
    }
  } catch {
    // Backend unavailable — show wizard anyway
  }

  return <OnboardingWizard />;
}

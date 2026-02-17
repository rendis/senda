import { redirect } from "next/navigation";
import { AppSidebar } from "@/components/layout/sidebar";
import { AppHeader } from "@/components/layout/header";
import type { OnboardingStatus } from "@/types/api";

async function checkOnboarding() {
  try {
    const apiUrl =
      process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
    const res = await fetch(`${apiUrl}/api/v1/onboarding/status`, {
      cache: "no-store",
    });
    if (!res.ok) return false;
    const data: OnboardingStatus = await res.json();
    return data.needs_onboarding;
  } catch {
    // Backend unavailable — skip check
    return false;
  }
}

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const needsOnboarding = await checkOnboarding();
  if (needsOnboarding) {
    redirect("/onboarding");
  }

  return (
    <div className="flex min-h-screen bg-page">
      <AppSidebar />
      <div className="flex flex-1 flex-col">
        <AppHeader />
        <main className="flex-1">{children}</main>
      </div>
    </div>
  );
}

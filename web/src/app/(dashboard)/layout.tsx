import { redirect } from "next/navigation";
import { auth } from "@/auth";
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

async function checkMembership(): Promise<boolean> {
  try {
    const apiUrl =
      process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
    const res = await fetch(`${apiUrl}/api/v1/members/me`, {
      cache: "no-store",
    });
    if (res.status === 403) return false;
    if (!res.ok) return true; // non-403 errors — skip check
    return true;
  } catch {
    // Backend unavailable — skip check
    return true;
  }
}

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // If token refresh failed, redirect to login so the user re-authenticates.
  // Can't call signIn() here because Server Components can't modify cookies.
  const session = await auth();
  if (session?.error === "RefreshTokenError") {
    redirect("/login");
  }

  const needsOnboarding = await checkOnboarding();
  if (needsOnboarding) {
    redirect("/onboarding");
  }

  const hasMembership = await checkMembership();
  if (!hasMembership) {
    redirect("/access-denied");
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

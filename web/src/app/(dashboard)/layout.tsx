import { redirect } from "next/navigation";
import { authWithoutRefresh } from "@/auth";
import { DashboardShell } from "@/components/layout/dashboard-shell";
import { MembershipGate } from "@/providers/membership-gate";

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Use authWithoutRefresh() — Server Components CANNOT write cookies,
  // so triggering token refresh here wastes the refreshed token and causes
  // double-refresh race conditions. Token refresh is handled by:
  // - proxy.ts (middleware) on navigation
  // - SessionProvider refetchInterval (every 3min) via /api/auth/session
  const session = await authWithoutRefresh();

  if (!session) {
    redirect("/login");
  }

  // Membership and onboarding checks are done client-side in MembershipGate
  // because they need a fresh idToken (from SessionProvider, which CAN update cookies).
  return (
    <DashboardShell>
      <MembershipGate>{children}</MembershipGate>
    </DashboardShell>
  );
}

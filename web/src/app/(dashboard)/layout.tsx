import { redirect } from "next/navigation";
import { headers } from "next/headers";
import { auth } from "@/auth";
import { DashboardShell } from "@/components/layout/dashboard-shell";
import type { OnboardingStatus } from "@/types/api";
import type { MemberWithRoles, MemberRoleDetail } from "@/types/members-ext";

function authHeaders(idToken?: string): HeadersInit | undefined {
  if (!idToken) {
    return undefined;
  }
  return {
    Authorization: `Bearer ${idToken}`,
  };
}

async function checkOnboarding(idToken?: string) {
  try {
    const apiUrl =
      process.env.SENDA_API_URL || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
    const res = await fetch(`${apiUrl}/api/v1/onboarding/status`, {
      cache: "no-store",
      headers: authHeaders(idToken),
    });
    if (!res.ok) return false;
    const data: OnboardingStatus = await res.json();
    return data.needs_onboarding;
  } catch {
    // Backend unavailable — skip check
    return false;
  }
}

async function fetchMembership(idToken?: string): Promise<MemberWithRoles | null | "unauthenticated"> {
  if (!idToken) {
    return "unauthenticated";
  }

  try {
    const apiUrl =
      process.env.SENDA_API_URL || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
    const res = await fetch(`${apiUrl}/api/v1/members/me`, {
      cache: "no-store",
      headers: authHeaders(idToken),
    });
    if (res.status === 401) {
      return "unauthenticated";
    }
    if (res.status === 403) {
      return null;
    }
    if (!res.ok) {
      return null;
    }
    return (await res.json()) as MemberWithRoles;
  } catch {
    return null;
  }
}

function requiredScopeForPath(pathname: string): "global" | "tenant" | "workspace" | null {
  if (pathname === "/global" || pathname.startsWith("/global/")) {
    return "global";
  }
  if (/^\/t\/[^/]+\/w\/[^/]+(?:\/|$)/.test(pathname)) {
    return "workspace";
  }
  if (/^\/t\/[^/]+(?:\/|$)/.test(pathname)) {
    return "tenant";
  }
  return null;
}

function hasScopeAccess(roles: MemberRoleDetail[], pathname: string): boolean {
  const scope = requiredScopeForPath(pathname);
  if (!scope) {
    return true;
  }

  const hasSuperadmin = roles.some((role) => role.role === "superadmin");
  if (hasSuperadmin) {
    return true;
  }

  switch (scope) {
    case "global":
      return false;
    case "tenant":
      return roles.some((role) => role.role === "tenant_admin");
    case "workspace":
      return roles.some(
        (role) =>
          role.role === "tenant_admin" ||
          role.role === "workspace_admin" ||
          role.role === "workspace_editor" ||
          role.role === "workspace_viewer"
      );
    default:
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

  const idToken = session?.idToken;
  const pathname =
    (await headers()).get("x-senda-pathname") ?? "";

  const needsOnboarding = await checkOnboarding(idToken);
  if (needsOnboarding) {
    redirect("/onboarding");
  }

  const membership = await fetchMembership(idToken);
  if (membership === "unauthenticated") {
    redirect("/login");
  }
  if (!membership || !hasScopeAccess(membership.roles ?? [], pathname)) {
    redirect("/access-denied");
  }

  return (
    <DashboardShell>{children}</DashboardShell>
  );
}

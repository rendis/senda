"use client";

import { useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { usePathname, useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import type { MemberWithRoles, MemberRoleDetail } from "@/types/members-ext";
import type { OnboardingStatus } from "@/types/api";

function requiredScopeForPath(pathname: string): "global" | "tenant" | "workspace" | null {
  if (pathname === "/global" || pathname.startsWith("/global/")) return "global";
  if (/^\/t\/[^/]+\/w\/[^/]+(?:\/|$)/.test(pathname)) return "workspace";
  if (/^\/t\/[^/]+(?:\/|$)/.test(pathname)) return "tenant";
  return null;
}

function hasScopeAccess(roles: MemberRoleDetail[], pathname: string): boolean {
  const scope = requiredScopeForPath(pathname);
  if (!scope) return true;
  if (roles.some((r) => r.role === "superadmin")) return true;
  switch (scope) {
    case "global": return false;
    case "tenant": return roles.some((r) => r.role === "tenant_admin");
    case "workspace":
      return roles.some((r) =>
        r.role === "tenant_admin" || r.role === "workspace_admin" ||
        r.role === "workspace_editor" || r.role === "workspace_viewer"
      );
    default: return true;
  }
}

/**
 * MembershipGate runs onboarding and membership checks CLIENT-SIDE
 * using the SessionProvider's fresh idToken (which CAN update cookies).
 *
 * This avoids the Server Component cookie bug where auth() in layout.tsx
 * triggers token refresh but cannot write the updated cookie back.
 */
export function MembershipGate({ children }: { children: React.ReactNode }) {
  const { data: session, status } = useSession();
  const pathname = usePathname();
  const router = useRouter();
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    setChecked(false);

    if (status !== "authenticated" || !session?.idToken) return;

    const idToken = session.idToken;
    let cancelled = false;

    async function check() {
      const headers = { Authorization: `Bearer ${idToken}` };

      try {
        const [onboardingRes, memberRes] = await Promise.all([
          fetch("/api/v1/onboarding/status", { headers }),
          fetch("/api/v1/members/me", { headers }),
        ]);

        if (cancelled) return;

        if (onboardingRes.ok) {
          const data: OnboardingStatus = await onboardingRes.json();
          if (data.needs_onboarding) {
            router.replace("/onboarding");
            return;
          }
        }

        if (memberRes.status === 401 || memberRes.status === 403) {
          router.replace("/access-denied");
          return;
        }
        if (memberRes.ok) {
          const member: MemberWithRoles = await memberRes.json();
          if (!hasScopeAccess(member.roles ?? [], pathname)) {
            router.replace("/access-denied");
            return;
          }
        } else {
          router.replace("/access-denied");
          return;
        }
      } catch {
        // Backend unavailable — allow through, individual pages will handle errors
      }

      if (!cancelled) setChecked(true);
    }

    check();
    return () => { cancelled = true; };
  }, [status, session?.idToken, pathname, router]);

  if (!checked) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return children;
}

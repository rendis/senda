"use client";

import { useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { usePathname, useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import type { MemberWithRoles, MemberRoleDetail } from "@/types/members-ext";
import type { OnboardingStatus, ScopeLevel } from "@/types/api";

interface ParsedScope {
  level: ScopeLevel | null;
  tenantCode?: string;
  workspaceCode?: string;
}

function parseScopePath(pathname: string): ParsedScope {
  const wsMatch = pathname.match(/^\/t\/([^/]+)\/w\/([^/]+)/);
  if (wsMatch) return { level: "workspace", tenantCode: wsMatch[1], workspaceCode: wsMatch[2] };

  const tenantMatch = pathname.match(/^\/t\/([^/]+)/);
  if (tenantMatch) return { level: "tenant", tenantCode: tenantMatch[1] };

  if (pathname === "/global" || pathname.startsWith("/global/")) return { level: "global" };

  return { level: null };
}

function hasScopeAccess(roles: MemberRoleDetail[], pathname: string): boolean {
  const { level, tenantCode, workspaceCode } = parseScopePath(pathname);
  if (!level) return true;

  // Only superadmin can access global scope
  if (level === "global") {
    return roles.some((r) => r.role === "superadmin" && r.scope_type === "global");
  }

  for (const r of roles) {
    if (r.role === "superadmin" && r.scope_type === "global") return true;

    // Tenant-scoped role covers tenant + all its workspaces
    if (r.scope_type === "tenant" && tenantCode && r.tenant_code === tenantCode) {
      return true;
    }

    // Workspace-scoped role must match exact workspace
    if (r.scope_type === "workspace" && workspaceCode && r.workspace_code === workspaceCode && level === "workspace") {
      return true;
    }
  }

  return false;
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
  const gateKey =
    status === "authenticated" && session?.idToken
      ? `${pathname}:${session.idToken}`
      : null;
  const [checkedKey, setCheckedKey] = useState<string | null>(null);

  useEffect(() => {
    if (!gateKey || !session?.idToken) return;

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

      if (!cancelled) setCheckedKey(gateKey);
    }

    check();
    return () => {
      cancelled = true;
    };
  }, [gateKey, pathname, router, session?.idToken]);

  if (!gateKey || checkedKey !== gateKey) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return children;
}

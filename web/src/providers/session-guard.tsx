"use client";

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { usePathname } from "next/navigation";
import { startFederatedLogout } from "@/lib/logout";
import { shouldTriggerFederatedLogout } from "@/providers/session-guard-policy";

/**
 * SessionGuard watches for expired/invalid sessions and redirects to login.
 *
 * Handles three scenarios:
 * 1. RefreshTokenError in session — token refresh failed, force logout
 * 2. pageshow with event.persisted — bfcache restoration after logout
 * 3. visibilitychange — tab becomes visible after being in background
 */
export function SessionGuard({ children }: { children: React.ReactNode }) {
  const { data: session, status } = useSession();
  const pathname = usePathname();
  const logoutTriggered = useRef(false);

  // 1. Watch for RefreshTokenError from server-side token refresh.
  useEffect(() => {
    if (
      shouldTriggerFederatedLogout({
        pathname,
        status,
        sessionError: session?.error,
        alreadyTriggered: logoutTriggered.current,
      })
    ) {
      logoutTriggered.current = true;
      startFederatedLogout("/login");
    }
  }, [pathname, status, session?.error]);

  // 2. Handle bfcache restoration (back button after logout).
  // 3. Handle tab visibility change (user returns to tab after long absence).
  useEffect(() => {
    function handlePageShow(event: PageTransitionEvent) {
      if (event.persisted && status === "unauthenticated") {
        window.location.replace("/login");
      }
    }

    function handleVisibilityChange() {
      if (
        document.visibilityState === "visible" &&
        status === "unauthenticated" &&
        !logoutTriggered.current
      ) {
        window.location.replace("/login");
      }
    }

    window.addEventListener("pageshow", handlePageShow);
    document.addEventListener("visibilitychange", handleVisibilityChange);

    return () => {
      window.removeEventListener("pageshow", handlePageShow);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [status]);

  return <>{children}</>;
}

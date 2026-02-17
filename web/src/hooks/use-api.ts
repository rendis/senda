"use client";

import { useSession } from "next-auth/react";
import { useMemo } from "react";
import { api, createAuthenticatedApi } from "@/lib/api";
import type { KyInstance } from "ky";

/**
 * Returns an authenticated ky instance using the current session's id_token.
 * Falls back to the base (unauthenticated) instance if no session.
 * Also returns `isReady` to indicate whether the session has loaded.
 */
export function useApi(): KyInstance {
  const { data: session } = useSession();
  const idToken = session?.idToken;

  return useMemo(() => {
    if (idToken) {
      return createAuthenticatedApi(idToken);
    }
    return api;
  }, [idToken]);
}

/**
 * Returns the session's id_token readiness.
 * Use this to gate queries that require authentication.
 * Returns false if the session has a refresh token error.
 */
export function useApiReady(): boolean {
  const { data: session, status } = useSession();
  return (
    status === "authenticated" &&
    !!session?.idToken &&
    !session?.error
  );
}

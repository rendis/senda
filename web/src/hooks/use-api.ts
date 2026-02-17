"use client";

import { useSession } from "next-auth/react";
import { api, createAuthenticatedApi } from "@/lib/api";
import type { KyInstance } from "ky";

/**
 * Returns an authenticated ky instance using the current session's id_token.
 * Falls back to the base (unauthenticated) instance if no session.
 */
export function useApi(): KyInstance {
  const { data: session } = useSession();
  const idToken = session?.idToken;

  if (idToken) {
    return createAuthenticatedApi(idToken);
  }
  return api;
}

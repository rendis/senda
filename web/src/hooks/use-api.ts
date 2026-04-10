"use client";

import { useSession } from "next-auth/react";
import { useMemo, useSyncExternalStore } from "react";
import { usePathname } from "next/navigation";
import { api, createAuthenticatedApi } from "@/lib/api";
import {
  EXTERNAL_EMBED_TOKEN_CHANGED_EVENT,
  isExternalEmbedPath,
  readExternalEmbedToken,
} from "@/lib/external-api-context";
import type { KyInstance } from "ky";

/**
 * Returns an authenticated ky instance using the current session's id_token.
 * Falls back to the base (unauthenticated) instance if no session.
 */
export function useApi(): KyInstance {
  const pathname = usePathname();
  const { data: session } = useSession();
  const idToken = session?.idToken;
  const externalEmbedReady = useExternalEmbedReady(pathname);

  return useMemo(() => {
    if (externalEmbedReady) {
      return api;
    }
    if (idToken) {
      return createAuthenticatedApi(idToken);
    }
    return api;
  }, [externalEmbedReady, idToken]);
}

export function useExternalEmbedReady(pathname: string): boolean {
  return useSyncExternalStore(
    (onStoreChange) => {
      if (typeof window === "undefined") {
        return () => undefined;
      }

      window.addEventListener(EXTERNAL_EMBED_TOKEN_CHANGED_EVENT, onStoreChange);
      window.addEventListener("storage", onStoreChange);

      return () => {
        window.removeEventListener(
          EXTERNAL_EMBED_TOKEN_CHANGED_EVENT,
          onStoreChange,
        );
        window.removeEventListener("storage", onStoreChange);
      };
    },
    () => {
      if (!isExternalEmbedPath(pathname)) {
        return false;
      }

      return readExternalEmbedToken() !== null;
    },
    () => false,
  );
}

/**
 * Returns the session's id_token readiness.
 * Use this to gate queries that require authentication.
 * Returns false if the session has a refresh token error.
 */
export function useApiReady(): boolean {
  const pathname = usePathname();
  const { data: session, status } = useSession();
  const externalReady = useExternalEmbedReady(pathname);

  if (externalReady) {
    return true;
  }

  return (
    status === "authenticated" &&
    !!session?.idToken &&
    !session?.error
  );
}

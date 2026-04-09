import ky, { type Options, type KyInstance, HTTPError } from "ky";
import type { ApiError } from "@/types/api";

/**
 * Base ky instance for Senda API.
 * Uses Next.js rewrites — requests go to /api/v1/* which proxies to backend.
 */
export const api = ky.create({
  prefix: "/api/v1",
  timeout: 30_000,
  hooks: {
    afterResponse: [
      async ({ request, response, retryCount }) => {
        if (
          response.status === 401 &&
          typeof window !== "undefined" &&
          request.headers.has("Authorization") &&
          retryCount === 0
        ) {
          // First 401 — attempt silent token refresh via next-auth session.
          const { getSession } = await import("next-auth/react");
          const session = await getSession();

          if (session?.idToken && !session.error) {
            const headers = new Headers(request.headers);
            headers.set("Authorization", `Bearer ${session.idToken}`);
            return ky.retry({
              request: new Request(request, { headers }),
              code: "TOKEN_REFRESHED",
            });
          }
        }

        // Retry also returned 401 — DON'T trigger federated logout.
        // The token may have expired between refresh and retry (race condition).
        // Let the SessionProvider refetch handle token renewal; individual
        // components should handle 401 as a transient error, not a logout signal.
        // Only trigger logout if the session itself reports RefreshTokenError
        // (handled by SessionGuard, not here).
        // 401 after retry — let error propagate. SessionGuard handles real auth failures.
      },
    ],
  },
});

/**
 * Create an authenticated ky instance with Bearer token.
 */
export function createAuthenticatedApi(idToken: string): KyInstance {
  return api.extend({
    hooks: {
      beforeRequest: [
        ({ request }) => {
          request.headers.set("Authorization", `Bearer ${idToken}`);
        },
      ],
    },
  });
}

/**
 * Extract a structured ApiError from an HTTPError.
 */
export async function parseApiError(error: unknown): Promise<ApiError> {
  if (error instanceof HTTPError) {
    try {
      return await error.response.json<ApiError>();
    } catch {
      return {
        error: {
          code: "UNKNOWN",
          message: error.message,
          request_id: error.response.headers.get("x-request-id") ?? undefined,
        },
      };
    }
  }
  return {
    error: {
      code: "NETWORK_ERROR",
      message: error instanceof Error ? error.message : "Unknown error",
    },
  };
}

/**
 * Type-safe GET helper.
 */
export async function apiGet<T>(path: string, options?: Options): Promise<T> {
  return api.get(path, options).json<T>();
}

/**
 * Type-safe POST helper.
 */
export async function apiPost<T>(
  path: string,
  body?: unknown,
  options?: Options
): Promise<T> {
  return api.post(path, { json: body, ...options }).json<T>();
}

/**
 * Type-safe PUT helper.
 */
export async function apiPut<T>(
  path: string,
  body?: unknown,
  options?: Options
): Promise<T> {
  return api.put(path, { json: body, ...options }).json<T>();
}

/**
 * Type-safe DELETE helper.
 */
export async function apiDelete<T>(
  path: string,
  options?: Options
): Promise<T> {
  return api.delete(path, options).json<T>();
}

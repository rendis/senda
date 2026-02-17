import ky, { type Options, type KyInstance, HTTPError } from "ky";
import type { ApiError } from "@/types/api";

/**
 * Base ky instance for Senda API.
 * Uses Next.js rewrites — requests go to /api/v1/* which proxies to backend.
 */
export const api = ky.create({
  prefixUrl: "/api/v1",
  timeout: 30_000,
  hooks: {
    afterResponse: [
      async (_request, _options, response) => {
        if (response.status === 401 && typeof window !== "undefined") {
          window.location.href = "/login";
        }
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
        (request) => {
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

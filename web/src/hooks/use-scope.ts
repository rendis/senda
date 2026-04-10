"use client";

import { useParams, useSearchParams } from "next/navigation";
import type { ScopeContext, ScopeLevel } from "@/types/api";
import { resolveScopedPathFromParams } from "@/lib/external-api-context";
import { normalizeEnvironment } from "@/lib/environment-mode";

/**
 * Extract scope context from URL params.
 *
 * URL patterns:
 *   /global/...              → level: "global"
 *   /t/[tenantCode]/...      → level: "tenant"
 *   /t/[tenantCode]/w/[workspaceCode]/... → level: "workspace"
 */
export function useScope(): ScopeContext {
  const params = useParams<{
    profileSlug?: string;
    environment?: string;
    tenantCode?: string;
    workspaceCode?: string;
  }>();
  const searchParams = useSearchParams();

  let level: ScopeLevel = "global";

  if (params.workspaceCode && params.tenantCode) {
    level = "workspace";
  } else if (params.tenantCode) {
    level = "tenant";
  }

  return {
    level,
    profileSlug: params.profileSlug,
    tenantCode: params.tenantCode,
    workspaceCode: params.workspaceCode,
    environment: normalizeEnvironment(
      params.environment ?? searchParams.get("environment"),
    ),
  };
}

/**
 * Build the management API base path for the current scope.
 */
export function useScopedPath(): string {
  return resolveScopedPathFromParams(useScope());
}

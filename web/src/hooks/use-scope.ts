"use client";

import { useParams } from "next/navigation";
import type { ScopeContext, ScopeLevel } from "@/types/api";
import { resolveScopedPathFromParams } from "@/lib/external-api-context";

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
    tenantCode?: string;
    workspaceCode?: string;
  }>();

  let level: ScopeLevel = "global";

  if (params.workspaceCode && params.tenantCode) {
    level = "workspace";
  } else if (params.tenantCode) {
    level = "tenant";
  }

  return {
    level,
    tenantCode: params.tenantCode,
    workspaceCode: params.workspaceCode,
  };
}

/**
 * Build the management API base path for the current scope.
 */
export function useScopedPath(): string {
  const params = useParams<{
    profileSlug?: string;
    tenantCode?: string;
    workspaceCode?: string;
  }>();

  return resolveScopedPathFromParams(params);
}

"use client";

import { useQuery } from "@tanstack/react-query";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import type { Tenant, Workspace, PaginatedResponse } from "@/types/api";

/**
 * Fetch all tenants (superadmin only).
 * Returns empty array on 403 (non-superadmin fallback).
 */
export function useTenantsQuery() {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["tenants"],
    queryFn: async () => {
      try {
        const res = await api
          .get("manage/tenants", { searchParams: { limit: 100 } })
          .json<PaginatedResponse<Tenant>>();
        return res.items;
      } catch {
        // 403 = non-superadmin, fall back to empty
        return [] as Tenant[];
      }
    },
    enabled: ready,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Fetch workspaces for a specific tenant.
 */
export function useWorkspacesQuery(tenantCode?: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["workspaces", tenantCode],
    queryFn: async () => {
      const res = await api
        .get(`manage/tenants/${tenantCode}/workspaces`, {
          searchParams: { limit: 100 },
        })
        .json<PaginatedResponse<Workspace>>();
      return res.items;
    },
    enabled: ready && !!tenantCode,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Paginated tenants with server-side search.
 */
export function usePaginatedTenants(search: string) {
  const api = useApi();
  const ready = useApiReady();

  return usePaginatedQuery<Tenant>({
    queryKey: ["tenants", "paginated", search],
    fetcher: async (cursor) => {
      const params: Record<string, string | number> = { limit: 25 };
      if (cursor) params.cursor = cursor;
      if (search) params.search = search;
      return api
        .get("manage/tenants", { searchParams: params })
        .json<PaginatedResponse<Tenant>>();
    },
    enabled: ready,
  });
}

/**
 * Paginated workspaces for a tenant with server-side search.
 */
export function usePaginatedWorkspaces(tenantCode: string, search: string) {
  const api = useApi();
  const ready = useApiReady();

  return usePaginatedQuery<Workspace>({
    queryKey: ["workspaces", tenantCode, "paginated", search],
    fetcher: async (cursor) => {
      const params: Record<string, string | number> = { limit: 25 };
      if (cursor) params.cursor = cursor;
      if (search) params.search = search;
      return api
        .get(`manage/tenants/${tenantCode}/workspaces`, { searchParams: params })
        .json<PaginatedResponse<Workspace>>();
    },
    enabled: ready && !!tenantCode,
  });
}

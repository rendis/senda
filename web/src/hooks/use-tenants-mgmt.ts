"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { PaginatedResponse, Tenant } from "@/types/api";

export interface CreateTenantInput {
  code: string;
  name: string;
}

export interface UpdateTenantInput {
  name?: string;
  is_active?: boolean;
}

function invalidateTenantQueries(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: ["tenants"] });
}

export function useTenants(search: string) {
  const api = useApi();
  const ready = useApiReady();

  return usePaginatedQuery<Tenant>({
    queryKey: ["tenants", "management", search],
    fetcher: (cursor) => {
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

export function useCreateTenant() {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTenantInput) =>
      api.post("manage/tenants", { json: data }).json<Tenant>(),
    onSuccess: () => {
      invalidateTenantQueries(queryClient);
    },
  });
}

export function useUpdateTenant() {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      tenantCode,
      data,
    }: {
      tenantCode: string;
      data: UpdateTenantInput;
    }) => api.put(`manage/tenants/${tenantCode}`, { json: data }).json<Tenant>(),
    onSuccess: () => {
      invalidateTenantQueries(queryClient);
    },
  });
}

export function useDeleteTenant() {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (tenantCode: string) => {
      await api.delete(`manage/tenants/${tenantCode}`);
    },
    onSuccess: () => {
      invalidateTenantQueries(queryClient);
    },
  });
}

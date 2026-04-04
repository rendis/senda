"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { PaginatedResponse, Workspace } from "@/types/api";

export interface CreateWorkspaceInput {
  code: string;
  name: string;
}

export interface UpdateWorkspaceInput {
  name?: string;
  is_active?: boolean;
}

function invalidateWorkspaceQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  tenantCode: string,
) {
  queryClient.invalidateQueries({ queryKey: ["workspaces", tenantCode] });
}

export function useWorkspacesManagement(tenantCode: string, search: string) {
  const api = useApi();
  const ready = useApiReady();

  return usePaginatedQuery<Workspace>({
    queryKey: ["workspaces", tenantCode, "management", search],
    fetcher: (cursor) => {
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

export function useCreateWorkspace(tenantCode: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateWorkspaceInput) =>
      api
        .post(`manage/tenants/${tenantCode}/workspaces`, { json: data })
        .json<Workspace>(),
    onSuccess: () => {
      invalidateWorkspaceQueries(queryClient, tenantCode);
    },
  });
}

export function useUpdateWorkspace(tenantCode: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      workspaceCode,
      data,
    }: {
      workspaceCode: string;
      data: UpdateWorkspaceInput;
    }) =>
      api
        .put(`manage/tenants/${tenantCode}/workspaces/${workspaceCode}`, {
          json: data,
        })
        .json<Workspace>(),
    onSuccess: () => {
      invalidateWorkspaceQueries(queryClient, tenantCode);
    },
  });
}

export function useDeleteWorkspace(tenantCode: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (workspaceCode: string) => {
      await api.delete(`manage/tenants/${tenantCode}/workspaces/${workspaceCode}`);
    },
    onSuccess: () => {
      invalidateWorkspaceQueries(queryClient, tenantCode);
    },
  });
}

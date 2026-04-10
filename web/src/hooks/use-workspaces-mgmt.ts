"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { Environment, PaginatedResponse, Workspace } from "@/types/api";
import { normalizeEnvironment } from "@/lib/environment-mode";

export interface CreateWorkspaceInput {
  code: string;
  name: string;
}

export interface UpdateWorkspaceInput {
  name?: string;
  is_active?: boolean;
  test_recipient_mode?: "replace" | "append";
  test_recipient_addresses?: string[];
}

function invalidateWorkspaceQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  tenantCode: string,
  environment?: Environment,
) {
  queryClient.invalidateQueries({
    queryKey: ["workspaces", tenantCode, normalizeEnvironment(environment)],
  });
}

export function useWorkspacesManagement(
  tenantCode: string,
  search: string,
  environment?: Environment,
) {
  const api = useApi();
  const ready = useApiReady();
  const normalizedEnvironment = normalizeEnvironment(environment);

  return usePaginatedQuery<Workspace>({
    queryKey: ["workspaces", tenantCode, normalizedEnvironment, "management", search],
    fetcher: (cursor) => {
      const params: Record<string, string | number> = { limit: 25 };
      if (cursor) params.cursor = cursor;
      if (search) params.search = search;
      return api
        .get(
          `manage/environments/${normalizedEnvironment}/tenants/${tenantCode}/workspaces`,
          { searchParams: params },
        )
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
      invalidateWorkspaceQueries(queryClient, tenantCode, "prod");
      invalidateWorkspaceQueries(queryClient, tenantCode, "test");
    },
  });
}

export function useUpdateWorkspace(
  tenantCode: string,
  environment?: Environment,
) {
  const api = useApi();
  const queryClient = useQueryClient();
  const normalizedEnvironment = normalizeEnvironment(environment);

  return useMutation({
    mutationFn: ({
      workspaceCode,
      data,
    }: {
      workspaceCode: string;
      data: UpdateWorkspaceInput;
    }) =>
      api
        .put(
          `manage/environments/${normalizedEnvironment}/tenants/${tenantCode}/workspaces/${workspaceCode}`,
          {
          json: data,
          },
        )
        .json<Workspace>(),
    onSuccess: () => {
      invalidateWorkspaceQueries(queryClient, tenantCode, normalizedEnvironment);
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
      invalidateWorkspaceQueries(queryClient, tenantCode, "prod");
      invalidateWorkspaceQueries(queryClient, tenantCode, "test");
    },
  });
}

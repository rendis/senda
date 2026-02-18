"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useScopedPath } from "@/hooks/use-scope";
import type { PaginatedResponse } from "@/types/api";
import type {
  ApiKey,
  ApiKeyCreateResponse,
  CreateApiKeyRequest,
} from "@/types/api-keys";

export function useApiKeys() {
  const api = useApi();
  const ready = useApiReady();
  const scopedPath = useScopedPath();
  const path = `${scopedPath}/api-keys`;

  return usePaginatedQuery<ApiKey>({
    queryKey: ["api-keys", scopedPath],
    fetcher: (cursor) =>
      api
        .get(path, { searchParams: cursor ? { cursor } : {} })
        .json<PaginatedResponse<ApiKey>>(),
    enabled: ready,
  });
}

export function useCreateApiKey() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateApiKeyRequest) =>
      api
        .post(`${scopedPath}/api-keys`, { json: data })
        .json<ApiKeyCreateResponse>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys", scopedPath] });
    },
  });
}

export function useRevokeApiKey() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      api.delete(`${scopedPath}/api-keys/${id}`).json<void>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys", scopedPath] });
    },
  });
}

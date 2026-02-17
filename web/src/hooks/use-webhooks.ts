"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useScopedPath } from "@/hooks/use-scope";
import type { PaginatedResponse } from "@/types/api";
import type {
  Webhook,
  CreateWebhookRequest,
  UpdateWebhookRequest,
  WebhookTestResult,
} from "@/types/webhooks";

export function useWebhooks() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const path = `${scopedPath}/webhooks`;

  return usePaginatedQuery<Webhook>({
    queryKey: ["webhooks", scopedPath],
    fetcher: (cursor) =>
      api
        .get(path, { searchParams: cursor ? { cursor } : {} })
        .json<PaginatedResponse<Webhook>>(),
  });
}

export function useCreateWebhook() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateWebhookRequest) =>
      api.post(`${scopedPath}/webhooks`, { json: data }).json<Webhook>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["webhooks", scopedPath] });
    },
  });
}

export function useUpdateWebhook() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateWebhookRequest }) =>
      api.put(`${scopedPath}/webhooks/${id}`, { json: data }).json<Webhook>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["webhooks", scopedPath] });
    },
  });
}

export function useDeleteWebhook() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      api.delete(`${scopedPath}/webhooks/${id}`).json<void>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["webhooks", scopedPath] });
    },
  });
}

export function useTestWebhook() {
  const api = useApi();
  const scopedPath = useScopedPath();

  return useMutation({
    mutationFn: (id: string) =>
      api
        .post(`${scopedPath}/webhooks/${id}/test`)
        .json<WebhookTestResult>(),
  });
}

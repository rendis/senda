"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { TemplateType } from "@/types/templates";
import type { PaginatedResponse } from "@/types/api";

export function useTemplateTypes(scopedPath: string) {
  const api = useApi();
  const ready = useApiReady();

  return usePaginatedQuery<TemplateType>({
    queryKey: ["template-types", scopedPath],
    fetcher: (cursor) =>
      api
        .get(`${scopedPath}/template-types`, {
          searchParams: cursor ? { cursor, limit: 50 } : { limit: 50 },
        })
        .json<PaginatedResponse<TemplateType>>(),
    enabled: ready,
  });
}

export function useTemplateType(scopedPath: string, slug: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["template-type", scopedPath, slug],
    queryFn: () =>
      api.get(`${scopedPath}/template-types/${slug}`).json<TemplateType>(),
    enabled: ready && !!slug,
  });
}

export function useUpdateTemplateType(scopedPath: string, slug: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { name?: string; slug?: string; adapter_id?: string; sender_identity_id?: string }) =>
      api
        .put(`${scopedPath}/template-types/${slug}`, { json: data })
        .json<TemplateType>(),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({
        queryKey: ["template-types", scopedPath],
      });
      queryClient.invalidateQueries({
        queryKey: ["template-type", scopedPath, slug],
      });
      if (updated.slug && updated.slug !== slug) {
        queryClient.invalidateQueries({
          queryKey: ["template-type", scopedPath, updated.slug],
        });
      }
    },
  });
}

export function useDeleteTemplateType(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slug: string) =>
      api.delete(`${scopedPath}/template-types/${slug}`).then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["template-types", scopedPath],
      });
    },
  });
}

export function useCreateTemplateType(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { slug: string; name: string; adapter_id?: string; sender_identity_id?: string }) =>
      api
        .post(`${scopedPath}/template-types`, { json: data })
        .json<TemplateType>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["template-types", scopedPath],
      });
    },
  });
}

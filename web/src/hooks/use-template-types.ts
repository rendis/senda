"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { TemplateType } from "@/types/templates";
import type { PaginatedResponse } from "@/types/api";

export function useTemplateTypes(scopedPath: string) {
  const api = useApi();

  return usePaginatedQuery<TemplateType>({
    queryKey: ["template-types", scopedPath],
    fetcher: (cursor) =>
      api
        .get(`${scopedPath}/template-types`, {
          searchParams: cursor ? { cursor, limit: 50 } : { limit: 50 },
        })
        .json<PaginatedResponse<TemplateType>>(),
  });
}

export function useTemplateType(scopedPath: string, slug: string) {
  const api = useApi();

  return useQuery({
    queryKey: ["template-type", scopedPath, slug],
    queryFn: () =>
      api.get(`${scopedPath}/template-types/${slug}`).json<TemplateType>(),
    enabled: !!slug,
  });
}

export function useCreateTemplateType(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { slug: string; name: string }) =>
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

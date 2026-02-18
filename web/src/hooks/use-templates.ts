"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import type { Template } from "@/types/templates";

export function useTemplatesByType(scopedPath: string, typeSlug: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["templates", scopedPath, typeSlug],
    queryFn: () =>
      api
        .get(`${scopedPath}/template-types/${typeSlug}/templates`)
        .json<{ items: Template[] }>()
        .then((r) => r.items),
    enabled: ready && !!typeSlug,
  });
}

export function useCreateTemplate(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { template_type_id: string }) =>
      api
        .post(`${scopedPath}/templates`, { json: data })
        .json<Template>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["templates", scopedPath] });
    },
  });
}

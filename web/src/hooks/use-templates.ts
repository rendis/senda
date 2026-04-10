"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { toast } from "sonner";
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

export function useDeleteTemplate(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (templateId: string) =>
      api.delete(`${scopedPath}/templates/${templateId}`).then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["templates", scopedPath] });
      toast.success("Template deleted");
    },
    onError: () => {
      toast.error("Cannot delete template (may have a published version)");
    },
  });
}

export function useForkTemplate(scopedPath: string, typeSlug: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (templateId: string) =>
      api.post(`${scopedPath}/templates/${templateId}/fork`).json<Template>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["templates", scopedPath, typeSlug] });
      queryClient.invalidateQueries({ queryKey: ["template-type", scopedPath, typeSlug] });
      toast.success("Default template forked into this workspace");
    },
    onError: () => {
      toast.error("Failed to fork default template");
    },
  });
}

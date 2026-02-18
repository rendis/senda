"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { PaginatedResponse } from "@/types/api";
import type {
  InjectorDefinition,
  InjectorWithValues,
  SetInjectorValuesRequest,
} from "@/types/injectors";
import { toast } from "sonner";

export function useInjectorList(scopedPath: string) {
  const api = useApi();
  const ready = useApiReady();

  return usePaginatedQuery<InjectorDefinition>({
    queryKey: ["injectors", scopedPath],
    fetcher: (cursor) => {
      const params = new URLSearchParams();
      if (cursor) params.set("cursor", cursor);
      const qs = params.toString();
      return api
        .get(`${scopedPath}/injectors${qs ? `?${qs}` : ""}`)
        .json<PaginatedResponse<InjectorDefinition>>();
    },
    enabled: ready,
  });
}

export function useInjectorDetail(scopedPath: string, name: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["injector", scopedPath, name],
    queryFn: () =>
      api.get(`${scopedPath}/injectors/${name}`).json<InjectorWithValues>(),
    enabled: ready && !!name,
  });
}

export function useSetInjectorValues(scopedPath: string, name: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SetInjectorValuesRequest) =>
      api
        .put(`${scopedPath}/injectors/${name}/values`, { json: data })
        .json<InjectorWithValues>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["injector", scopedPath, name] });
      toast.success("Injector values saved");
    },
    onError: () => {
      toast.error("Failed to save injector values");
    },
  });
}

export function useDeleteInjectorOverride(scopedPath: string, name: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (fieldName: string) =>
      api
        .delete(`${scopedPath}/injectors/${name}/values/${fieldName}`)
        .then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["injector", scopedPath, name] });
      toast.success("Override removed");
    },
    onError: () => {
      toast.error("Failed to remove override");
    },
  });
}

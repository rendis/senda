"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import type {
  CreateInjectorRequest,
  InjectorDefinition,
  InjectorField,
  InjectorListResponse,
  UpdateInjectorRequest,
  UpdateInjectorFieldRequest,
} from "@/types/injectors";
import { toast } from "sonner";

export interface UseInjectorListOptions {
  enabled?: boolean;
  includeInherited?: boolean;
}

export function buildInjectorListPath(
  scopedPath: string,
  options: UseInjectorListOptions = {},
): string {
  const params = new URLSearchParams();
  if (options.includeInherited) {
    params.set("include_inherited", "true");
  }

  const query = params.toString();
  return query ? `${scopedPath}/injectors?${query}` : `${scopedPath}/injectors`;
}

export function useInjectorList(
  scopedPath: string,
  options: UseInjectorListOptions = {},
) {
  const api = useApi();
  const ready = useApiReady();
  const { enabled = true, includeInherited = false } = options;

  return useQuery({
    queryKey: ["injectors", scopedPath, includeInherited],
    queryFn: () =>
      api.get(buildInjectorListPath(scopedPath, { includeInherited })).json<InjectorListResponse>(),
    enabled: ready && enabled,
  });
}

export function useInjectorDetail(scopedPath: string, name: string, enabled = true) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["injector", scopedPath, name],
    queryFn: () => api.get(`${scopedPath}/injectors/${name}`).json<InjectorDefinition>(),
    enabled: ready && enabled && !!name,
  });
}

export function useCreateInjector(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateInjectorRequest) =>
      api.post(`${scopedPath}/injectors`, { json: data }).json<InjectorDefinition>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["injectors", scopedPath] });
      toast.success("Injector created");
    },
    onError: () => {
      toast.error("Failed to create injector");
    },
  });
}

export function useUpdateInjector(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      currentName,
      data,
    }: {
      currentName: string;
      data: UpdateInjectorRequest;
    }) =>
      api
        .put(`${scopedPath}/injectors/${currentName}`, { json: data })
        .json<InjectorDefinition>(),
    onSuccess: (updated, variables) => {
      queryClient.invalidateQueries({ queryKey: ["injectors", scopedPath] });
      queryClient.invalidateQueries({
        queryKey: ["injector", scopedPath, variables.currentName],
      });
      queryClient.invalidateQueries({
        queryKey: ["injector", scopedPath, updated.name],
      });
      toast.success("Injector updated");
    },
    onError: () => {
      toast.error("Failed to update injector");
    },
  });
}

export function useUpdateInjectorField(scopedPath: string, injectorName: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      fieldName,
      data,
    }: {
      fieldName: string;
      data: UpdateInjectorFieldRequest;
    }) =>
      api
        .put(`${scopedPath}/injectors/${injectorName}/fields/${fieldName}`, {
          json: data,
        })
        .json<InjectorField>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["injector", scopedPath, injectorName] });
      queryClient.invalidateQueries({ queryKey: ["injectors", scopedPath] });
      toast.success("Field updated");
    },
    onError: () => {
      toast.error("Failed to update field");
    },
  });
}

export function useDeleteInjector(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => api.delete(`${scopedPath}/injectors/${name}`).then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["injectors", scopedPath] });
      toast.success("Injector deleted");
    },
    onError: () => {
      toast.error("Failed to delete injector");
    },
  });
}

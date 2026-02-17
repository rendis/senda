"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { PaginatedResponse } from "@/types/api";
import type {
  Adapter,
  CreateAdapterRequest,
  UpdateAdapterRequest,
} from "@/types/adapters";
import { toast } from "sonner";

export function useAdapterList(scopedPath: string) {
  const api = useApi();

  return usePaginatedQuery<Adapter>({
    queryKey: ["adapters", scopedPath],
    fetcher: (cursor) => {
      const params = new URLSearchParams();
      if (cursor) params.set("cursor", cursor);
      const qs = params.toString();
      return api
        .get(`${scopedPath}/adapters${qs ? `?${qs}` : ""}`)
        .json<PaginatedResponse<Adapter>>();
    },
  });
}

export function useAdapterDetail(scopedPath: string, id: string) {
  const api = useApi();

  return useQuery({
    queryKey: ["adapter", scopedPath, id],
    queryFn: () => api.get(`${scopedPath}/adapters/${id}`).json<Adapter>(),
    enabled: !!id,
  });
}

export function useCreateAdapter(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateAdapterRequest) =>
      api.post(`${scopedPath}/adapters`, { json: data }).json<Adapter>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["adapters", scopedPath] });
      toast.success("Adapter created");
    },
    onError: () => {
      toast.error("Failed to create adapter");
    },
  });
}

export function useUpdateAdapter(scopedPath: string, id: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateAdapterRequest) =>
      api.put(`${scopedPath}/adapters/${id}`, { json: data }).json<Adapter>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["adapters", scopedPath] });
      queryClient.invalidateQueries({ queryKey: ["adapter", scopedPath, id] });
      toast.success("Adapter updated");
    },
    onError: () => {
      toast.error("Failed to update adapter");
    },
  });
}

export function useDeleteAdapter(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      api.delete(`${scopedPath}/adapters/${id}`).then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["adapters", scopedPath] });
      toast.success("Adapter deleted");
    },
    onError: () => {
      toast.error("Failed to delete adapter");
    },
  });
}

export function useTestAdapterConnection(scopedPath: string, id: string) {
  const api = useApi();

  return useMutation({
    mutationFn: () =>
      api
        .post(`${scopedPath}/adapters/${id}/test`)
        .json<{ success: boolean; message?: string }>(),
    onSuccess: (data) => {
      if (data.success) {
        toast.success("Connection successful");
      } else {
        toast.error(data.message ?? "Connection failed");
      }
    },
    onError: () => {
      toast.error("Connection test failed");
    },
  });
}

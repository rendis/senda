"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { PaginatedResponse } from "@/types/api";
import type { Domain, RegisterDomainRequest } from "@/types/domains";
import { toast } from "sonner";

export function useDomainList(scopedPath: string) {
  const api = useApi();
  const ready = useApiReady();

  return usePaginatedQuery<Domain>({
    queryKey: ["domains", scopedPath],
    fetcher: (cursor) => {
      const params = new URLSearchParams();
      if (cursor) params.set("cursor", cursor);
      const qs = params.toString();
      return api
        .get(`${scopedPath}/domains${qs ? `?${qs}` : ""}`)
        .json<PaginatedResponse<Domain>>();
    },
    enabled: ready,
  });
}

export function useDomainDetail(scopedPath: string, id: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["domain", scopedPath, id],
    queryFn: () => api.get(`${scopedPath}/domains/${id}`).json<Domain>(),
    enabled: ready && !!id,
  });
}

export function useRegisterDomain(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: RegisterDomainRequest) =>
      api.post(`${scopedPath}/domains`, { json: data }).json<Domain>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["domains", scopedPath] });
      toast.success("Domain registered");
    },
    onError: () => {
      toast.error("Failed to register domain");
    },
  });
}

export function useDeleteDomain(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      api.delete(`${scopedPath}/domains/${id}`).then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["domains", scopedPath] });
      toast.success("Domain removed");
    },
    onError: () => {
      toast.error("Failed to remove domain");
    },
  });
}

export function useVerifyDomain(scopedPath: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      api.post(`${scopedPath}/domains/${id}/verify`).json<Domain>(),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["domains", scopedPath] });
      queryClient.invalidateQueries({
        queryKey: ["domain", scopedPath, data.id],
      });
      if (data.status === "verified") {
        toast.success("Domain verified successfully");
      } else {
        toast.warning("Verification in progress, check back later");
      }
    },
    onError: () => {
      toast.error("Domain verification failed");
    },
  });
}

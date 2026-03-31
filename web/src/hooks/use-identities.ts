"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import type { AdapterIdentity } from "@/types/adapters";
import { toast } from "sonner";

export function useIdentityList(scopedPath: string, adapterId: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["identities", scopedPath, adapterId],
    queryFn: () =>
      api
        .get(`${scopedPath}/adapters/${adapterId}/identities`)
        .json<AdapterIdentity[]>(),
    enabled: ready && !!adapterId,
  });
}

export function useSyncIdentities(scopedPath: string, adapterId: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () =>
      api
        .post(`${scopedPath}/adapters/${adapterId}/identities/sync`)
        .json<AdapterIdentity[]>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["identities", scopedPath, adapterId],
      });
      toast.success("Identities synced from provider");
    },
    onError: () => {
      toast.error("Failed to sync identities");
    },
  });
}

export function useCreateIdentity(scopedPath: string, adapterId: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { identity: string; display_name?: string }) =>
      api
        .post(`${scopedPath}/adapters/${adapterId}/identities`, { json: data })
        .json<AdapterIdentity>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["identities", scopedPath, adapterId],
      });
      toast.success("Identity added");
    },
    onError: () => {
      toast.error("Failed to add identity");
    },
  });
}

export function useDeleteIdentity(scopedPath: string, adapterId: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (identityId: string) =>
      api
        .delete(
          `${scopedPath}/adapters/${adapterId}/identities/${identityId}`
        )
        .then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["identities", scopedPath, adapterId],
      });
      toast.success("Identity removed");
    },
    onError: () => {
      toast.error("Failed to remove identity");
    },
  });
}

export function useSetDefaultIdentity(
  scopedPath: string,
  adapterId: string
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (identityId: string) =>
      api
        .post(
          `${scopedPath}/adapters/${adapterId}/identities/${identityId}/set-default`
        )
        .then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["identities", scopedPath, adapterId],
      });
      toast.success("Default sender updated");
    },
    onError: () => {
      toast.error("Failed to set default");
    },
  });
}

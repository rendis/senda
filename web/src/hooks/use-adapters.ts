"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { PaginatedResponse } from "@/types/api";
import type {
  Adapter,
  CreateAdapterRequest,
  UpdateAdapterRequest,
  ProvisioningStatusResponse,
} from "@/types/adapters";
import { toast } from "sonner";

export function useAdapterList(scopedPath: string) {
  const api = useApi();
  const ready = useApiReady();

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
    enabled: ready,
  });
}

export function useAdapterDetail(scopedPath: string, id: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["adapter", scopedPath, id],
    queryFn: () => api.get(`${scopedPath}/adapters/${id}`).json<Adapter>(),
    enabled: ready && !!id,
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

export interface TestAdapterRequest {
  to: string;
  subject: string;
  body: string;
}

export interface TestAdapterResponse {
  status: string;
  provider_message_id: string;
  from: string;
}

export function useTestAdapterSend(scopedPath: string, id: string) {
  const api = useApi();

  return useMutation({
    mutationFn: (data: TestAdapterRequest) =>
      api
        .post(`${scopedPath}/adapters/${id}/test`, { json: data })
        .json<TestAdapterResponse>(),
    onSuccess: (data) => {
      toast.success(`Test email sent from ${data.from}`);
    },
    onError: () => {
      toast.error("Test send failed");
    },
  });
}

export interface SESValidationCheck {
  name: string;
  permission: string;
  status: "ok" | "denied" | "error";
  description: string;
  required: boolean;
}

export interface SESValidationResult {
  valid: boolean;
  checks: SESValidationCheck[];
}

export function useProvisioningStatus(
  scopedPath: string,
  id: string,
  enabled: boolean
) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["provisioning-status", scopedPath, id],
    queryFn: () =>
      api
        .get(`${scopedPath}/adapters/${id}/provisioning-status`)
        .json<ProvisioningStatusResponse>(),
    enabled: ready && !!id && enabled,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "in_progress" ? 1000 : false;
    },
  });
}

export function useAutoProvision(scopedPath: string, id: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () =>
      api
        .post(`${scopedPath}/adapters/${id}/auto-provision-tracking`)
        .json<Record<string, unknown>>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["provisioning-status", scopedPath, id],
      });
    },
    onError: () => {
      toast.error("Provisioning failed");
    },
  });
}

export function useValidateSES(scopedPath: string) {
  const api = useApi();

  return useMutation({
    mutationFn: (data: { region: string; access_key_id: string; secret_access_key: string }) =>
      api
        .post(`${scopedPath}/adapters/validate-ses`, { json: data })
        .json<SESValidationResult>(),
  });
}

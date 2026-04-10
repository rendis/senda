"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { useParams } from "next/navigation";
import {
  canManageSystemWorkspacePolicies,
  isWorkspaceScope,
} from "@/lib/workspace-resource-policies";
import { resolveWorkspacePoliciesPathFromParams } from "@/lib/external-api-context";
import { type ScopeContext } from "@/types/api";
import type {
  SystemSettings,
  UpdateSettingsRequest,
  UpdateWorkspacePoliciesRequest,
  WorkspacePolicies,
} from "@/types/settings";

function buildWorkspacePoliciesPath(tenantCode: string, workspaceCode: string) {
  return `manage/tenants/${tenantCode}/workspaces/${workspaceCode}/policies`;
}

export function useSettings() {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["settings"],
    queryFn: () => api.get("manage/config").json<SystemSettings>(),
    enabled: ready,
  });
}

export function useUpdateSettings() {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateSettingsRequest) =>
      api.put("manage/config", { json: data }).json<SystemSettings>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
  });
}

export function useWorkspacePolicies(
  tenantCode?: string,
  workspaceCode?: string,
  enabled = true,
) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["workspace-policies", tenantCode, workspaceCode],
    queryFn: () =>
      api
        .get(buildWorkspacePoliciesPath(tenantCode!, workspaceCode!))
        .json<WorkspacePolicies>(),
    enabled: ready && enabled && !!tenantCode && !!workspaceCode,
    retry: false,
  });
}

export function useUpdateWorkspacePolicies(
  tenantCode?: string,
  workspaceCode = "_system",
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateWorkspacePoliciesRequest) =>
      api
        .put(buildWorkspacePoliciesPath(tenantCode!, workspaceCode), {
          json: data,
        })
        .json<WorkspacePolicies>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["workspace-policies", tenantCode, workspaceCode],
      });
    },
  });
}

export function useResolvedWorkspacePolicies(scope: ScopeContext) {
  const params = useParams<{
    profileSlug?: string;
    tenantCode?: string;
    workspaceCode?: string;
  }>();
  const policiesPath = resolveWorkspacePoliciesPathFromParams(params);
  const api = useApi();
  const ready = useApiReady();

  const query = useQuery({
    queryKey: ["workspace-policies", policiesPath],
    queryFn: () => api.get(policiesPath!).json<WorkspacePolicies>(),
    enabled: ready && isWorkspaceScope(scope) && !!policiesPath,
    retry: false,
  });

  return {
    ...query,
    data: query.data,
    canManage: canManageSystemWorkspacePolicies(scope),
  };
}

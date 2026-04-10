"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import {
  canManageSystemWorkspacePolicies,
  isWorkspaceScope,
} from "@/lib/workspace-resource-policies";
import { resolveWorkspacePoliciesPathFromParams } from "@/lib/external-api-context";
import { type Environment, type ScopeContext } from "@/types/api";
import { normalizeEnvironment } from "@/lib/environment-mode";
import { useScope } from "@/hooks/use-scope";
import type {
  SystemSettings,
  UpdateSettingsRequest,
  UpdateWorkspacePoliciesRequest,
  WorkspacePolicies,
} from "@/types/settings";

function buildWorkspacePoliciesPath(
  tenantCode: string,
  workspaceCode: string,
  environment?: Environment,
) {
  return `manage/environments/${normalizeEnvironment(environment)}/tenants/${tenantCode}/workspaces/${workspaceCode}/policies`;
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
  environment?: Environment,
  enabled = true,
) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["workspace-policies", tenantCode, workspaceCode, environment],
    queryFn: () =>
      api
        .get(buildWorkspacePoliciesPath(tenantCode!, workspaceCode!, environment))
        .json<WorkspacePolicies>(),
    enabled: ready && enabled && !!tenantCode && !!workspaceCode,
    retry: false,
  });
}

export function useUpdateWorkspacePolicies(
  tenantCode?: string,
  workspaceCode = "_system",
  environment?: Environment,
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateWorkspacePoliciesRequest) =>
      api
        .put(buildWorkspacePoliciesPath(tenantCode!, workspaceCode, environment), {
          json: data,
        })
        .json<WorkspacePolicies>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["workspace-policies", tenantCode, workspaceCode, environment],
      });
    },
  });
}

export function useResolvedWorkspacePolicies(scope: ScopeContext) {
  const resolvedScope = useScope();
  const policiesPath = resolveWorkspacePoliciesPathFromParams({
    ...resolvedScope,
    ...scope,
  });
  const api = useApi();
  const ready = useApiReady();

  const query = useQuery({
    queryKey: ["workspace-policies", policiesPath, resolvedScope.environment],
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

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useScope } from "@/hooks/use-scope";
import type { PaginatedResponse } from "@/types/api";
import type {
  MemberWithRoles,
  InviteMemberRequest,
  AddMemberRoleRequest,
} from "@/types/members-ext";

function useMembersPath(): string {
  const { level, tenantCode, workspaceCode } = useScope();

  switch (level) {
    case "tenant":
      return `manage/tenants/${tenantCode}/members`;
    case "workspace":
      return `manage/tenants/${tenantCode}/workspaces/${workspaceCode}/members`;
    default:
      return "manage/members";
  }
}

export function useCurrentMember() {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["members", "me"],
    queryFn: () => api.get("members/me").json<MemberWithRoles>(),
    enabled: ready,
    staleTime: 60_000,
  });
}

export function useMembers() {
  const api = useApi();
  const ready = useApiReady();
  const path = useMembersPath();

  return usePaginatedQuery<MemberWithRoles>({
    queryKey: ["members", path],
    fetcher: (cursor) =>
      api
        .get(path, { searchParams: cursor ? { cursor } : {} })
        .json<PaginatedResponse<MemberWithRoles>>(),
    enabled: ready,
  });
}

export function useInviteMember() {
  const api = useApi();
  const qc = useQueryClient();
  const path = useMembersPath();

  return useMutation({
    mutationFn: (data: InviteMemberRequest) =>
      api.post(path, { json: data }).json<MemberWithRoles>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

export function useAddMemberRole() {
  const api = useApi();
  const qc = useQueryClient();
  const path = useMembersPath();

  return useMutation({
    mutationFn: ({
      memberId,
      data,
    }: {
      memberId: string;
      data: AddMemberRoleRequest;
    }) =>
      api
        .post(`${path}/${memberId}/roles`, { json: data })
        .json<MemberWithRoles>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

export function useRemoveMemberRole() {
  const api = useApi();
  const qc = useQueryClient();
  const path = useMembersPath();

  return useMutation({
    mutationFn: ({
      memberId,
      roleId,
    }: {
      memberId: string;
      roleId: string;
    }) =>
      api.delete(`${path}/${memberId}/roles/${roleId}`).json<void>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

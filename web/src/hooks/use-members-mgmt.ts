"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import type { PaginatedResponse } from "@/types/api";
import type {
  MemberWithRoles,
  InviteMemberRequest,
  AddMemberRoleRequest,
} from "@/types/members-ext";

export function useMembers() {
  const api = useApi();
  const ready = useApiReady();
  const path = "manage/members";

  return usePaginatedQuery<MemberWithRoles>({
    queryKey: ["members"],
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

  return useMutation({
    mutationFn: (data: InviteMemberRequest) =>
      api
        .post("manage/members", { json: data })
        .json<MemberWithRoles>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

export function useAddMemberRole() {
  const api = useApi();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({
      memberId,
      data,
    }: {
      memberId: string;
      data: AddMemberRoleRequest;
    }) =>
      api
        .post(`manage/members/${memberId}/roles`, { json: data })
        .json<MemberWithRoles>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

export function useRemoveMemberRole() {
  const api = useApi();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({
      memberId,
      roleId,
    }: {
      memberId: string;
      roleId: string;
    }) =>
      api
        .delete(`manage/members/${memberId}/roles/${roleId}`)
        .json<void>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

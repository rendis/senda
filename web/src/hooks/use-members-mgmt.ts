"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useScopedPath } from "@/hooks/use-scope";
import type { PaginatedResponse } from "@/types/api";
import type {
  MemberWithRoles,
  InviteMemberRequest,
  AddMemberRoleRequest,
} from "@/types/members-ext";

export function useMembers() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const path = `${scopedPath}/members`;

  return usePaginatedQuery<MemberWithRoles>({
    queryKey: ["members", scopedPath],
    fetcher: (cursor) =>
      api
        .get(path, { searchParams: cursor ? { cursor } : {} })
        .json<PaginatedResponse<MemberWithRoles>>(),
  });
}

export function useInviteMember() {
  const api = useApi();
  const scopedPath = useScopedPath();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (data: InviteMemberRequest) =>
      api
        .post(`${scopedPath}/members`, { json: data })
        .json<MemberWithRoles>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members", scopedPath] });
    },
  });
}

export function useAddMemberRole() {
  const api = useApi();
  const scopedPath = useScopedPath();
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
        .post(`${scopedPath}/members/${memberId}/roles`, { json: data })
        .json<MemberWithRoles>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members", scopedPath] });
    },
  });
}

export function useRemoveMemberRole() {
  const api = useApi();
  const scopedPath = useScopedPath();
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
        .delete(`${scopedPath}/members/${memberId}/roles/${roleId}`)
        .json<void>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members", scopedPath] });
    },
  });
}

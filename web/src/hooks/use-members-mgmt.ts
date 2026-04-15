"use client";

import type { InfiniteData } from "@tanstack/react-query";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useScope } from "@/hooks/use-scope";
import type { PaginatedResponse } from "@/types/api";
import type {
  MemberWithRoles,
  InviteMemberRequest,
  ReplaceMemberRoleRequest,
} from "@/types/members-ext";
import {
  buildMemberAccessPath,
  buildMemberRolePath,
  buildMembersPath,
  performNoContentRequest,
  removeMemberFromCachedPages,
} from "./members-mgmt-logic";

export {
  buildInviteMemberRequest,
  buildMemberAccessPath,
  buildMemberRolePath,
  buildMembersPath,
  buildRevokeAccessDialogCopy,
  getAllowedMemberRolesForScope,
  hasMemberAccessInScope,
  getMemberRowActions,
  inviteMemberInScope,
} from "./members-mgmt-logic";

function useMembersPath(): string {
  return buildMembersPath(useScope());
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

export function useReplaceMemberRole() {
  const api = useApi();
  const qc = useQueryClient();
  const scope = useScope();

  return useMutation({
    mutationFn: ({
      memberId,
      data,
    }: {
      memberId: string;
      data: ReplaceMemberRoleRequest;
    }) =>
      api.put(buildMemberRolePath(scope, memberId), { json: data }).json<MemberWithRoles>(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

export function useRevokeMemberAccess() {
  const api = useApi();
  const qc = useQueryClient();
  const scope = useScope();
  const path = buildMembersPath(scope);

  return useMutation({
    mutationFn: async (memberId: string) =>
      performNoContentRequest(() =>
        api.delete(buildMemberAccessPath(scope, memberId)),
      ),
    onMutate: async (memberId) => {
      await qc.cancelQueries({ queryKey: ["members", path] });
      const previous =
        qc.getQueryData<InfiniteData<PaginatedResponse<MemberWithRoles>>>([
          "members",
          path,
        ]);

      qc.setQueryData<InfiniteData<PaginatedResponse<MemberWithRoles>>>(
        ["members", path],
        (current) => removeMemberFromCachedPages(current, memberId),
      );

      return { previous };
    },
    onError: (_error, _memberId, context) => {
      if (!context?.previous) {
        return;
      }

      qc.setQueryData(["members", path], context.previous);
    },
    onSettled: async () => {
      await qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

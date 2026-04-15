"use client";

import { useEffect, useMemo, useState } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { UserPlus, Users, ShieldCheck, Trash2, Search } from "lucide-react";
import { toast } from "sonner";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { RoleBadge } from "./role-badge";
import { MemberScopeBadge } from "./scope-badge";
import { InviteMemberForm } from "./invite-member-form";
import { RoleEditor } from "./role-editor";
import {
  useCurrentMember,
  useMembers,
  useInviteMember,
  useReplaceMemberRole,
  useRevokeMemberAccess,
  buildRevokeAccessDialogCopy,
  getMemberRowActions,
  getAllowedMemberRolesForScope,
  hasMemberAccessInScope,
  inviteMemberInScope,
} from "@/hooks/use-members-mgmt";
import {
  getMemberRolesInScope,
  getPrimaryMemberRoleInScope,
} from "@/hooks/members-mgmt-logic";
import { useScope } from "@/hooks/use-scope";
import {
  SYSTEM_WORKSPACE_SCOPE_LABEL,
  isSystemWorkspaceCode,
} from "@/lib/system-workspace-display";
import type { MemberWithRoles, MemberRoleDetail } from "@/types/members-ext";
import type { Role, ScopeLevel } from "@/types/api";

function canManageMembers(roles: MemberRoleDetail[], scopeLevel: ScopeLevel): boolean {
  if (roles.some((r) => r.role === "superadmin")) return true;
  switch (scopeLevel) {
    case "tenant":
      return roles.some((r) => r.role === "tenant_admin");
    case "workspace":
      return roles.some(
        (r) => r.role === "tenant_admin" || r.role === "workspace_admin",
      );
    case "global":
    default:
      return false;
  }
}

function scopeLabel(scopeLevel: ScopeLevel, tenantCode?: string, workspaceCode?: string): string {
  switch (scopeLevel) {
    case "tenant":
      return tenantCode ? `tenant "${tenantCode}"` : "this tenant";
    case "workspace":
      if (!workspaceCode) return "this workspace";
      return isSystemWorkspaceCode(workspaceCode)
        ? `the ${SYSTEM_WORKSPACE_SCOPE_LABEL}`
        : `workspace "${workspaceCode}"`;
    case "global":
    default:
      return "global scope";
  }
}

export function MembersContent() {
  const scope = useScope();
  const { data: currentMember } = useCurrentMember();
  const canManage = canManageMembers(currentMember?.roles ?? [], scope.level);

  return (
    <MembersTable
      canManage={canManage}
      scopeLevel={scope.level}
      tenantCode={scope.tenantCode}
      workspaceCode={scope.workspaceCode}
    />
  );
}

function MembersTable({
  canManage,
  scopeLevel,
  tenantCode,
  workspaceCode,
}: {
  canManage: boolean;
  scopeLevel: ScopeLevel;
  tenantCode?: string;
  workspaceCode?: string;
}) {
  const { data, isLoading, error, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useMembers();
  const inviteMutation = useInviteMember();
  const replaceRoleMutation = useReplaceMemberRole();
  const revokeAccessMutation = useRevokeMemberAccess();

  const [inviteOpen, setInviteOpen] = useState(false);
  const [roleEditorTarget, setRoleEditorTarget] =
    useState<MemberWithRoles | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<MemberWithRoles | null>(null);
  const [searchQuery, setSearchQuery] = useState("");

  useEffect(() => {
    if (error) toast.error("Failed to load members");
  }, [error]);

  const members = useMemo(() => {
    const allMembers = data?.pages.flatMap((p) => p.items) ?? [];
    const visibleMembers = allMembers.filter((member) =>
      hasMemberAccessInScope(member, {
        level: scopeLevel,
        tenantCode,
        workspaceCode,
      }),
    );
    if (!searchQuery) return visibleMembers;
    return visibleMembers.filter(
      (m) =>
        m.email.toLowerCase().includes(searchQuery.toLowerCase()) ||
        m.display_name?.toLowerCase().includes(searchQuery.toLowerCase())
    );
  }, [data?.pages, scopeLevel, searchQuery, tenantCode, workspaceCode]);

  const handleInvite = async (formData: {
    email: string;
    display_name?: string;
    role: Role;
  }) => {
    const result = await inviteMemberInScope({
      scopeLevel,
      formData,
      inviteMember: (payload) => inviteMutation.mutateAsync(payload),
      replaceMemberRole: ({ memberId, data }) =>
        replaceRoleMutation.mutateAsync({
          memberId,
          data,
        }),
    });

    setInviteOpen(false);

    if (result.status === "needs-role-retry") {
      setRoleEditorTarget(result.member);
      toast.error(
        "Member created, but assigning the initial role failed. Finish setup in Change role.",
      );
      return;
    }

    toast.success("Member invited");
  };

  const handleRevokeAccess = async () => {
    if (!revokeTarget) return;
    await revokeAccessMutation.mutateAsync(revokeTarget.id);
    setRevokeTarget(null);
    toast.success("Access revoked");
  };

  const handleChangeRole = async (
    memberId: string,
    data: { role: Role; scope_type: ScopeLevel }
  ) => {
    await replaceRoleMutation.mutateAsync({
      memberId,
      data,
    });
    setRoleEditorTarget(null);
    toast.success("Role updated");
  };

  const rowActions = getMemberRowActions(scopeLevel);
  const revokeDialogCopy = buildRevokeAccessDialogCopy({
    memberEmail: revokeTarget?.email ?? "",
    scopeLabel: scopeLabel(scopeLevel, tenantCode, workspaceCode),
  });

  const columns: ColumnDef<MemberWithRoles>[] = [
    {
      accessorKey: "email",
      header: "Email",
      cell: ({ row }) => (
        <span className="font-mono text-[13px]">{row.original.email}</span>
      ),
    },
    {
      accessorKey: "display_name",
      header: "Name",
      size: 160,
      cell: ({ row }) => (
        <span className="text-[13px]">{row.original.display_name}</span>
      ),
    },
    {
      id: "role",
      header: "Role",
      size: 160,
      enableSorting: false,
      cell: ({ row }) => {
        const primaryRole = getPrimaryMemberRoleInScope(row.original, {
          level: scopeLevel,
          tenantCode,
          workspaceCode,
        });
        return primaryRole ? (
          <RoleBadge role={primaryRole.role} />
        ) : (
          <span className="text-xs text-muted-foreground">No role</span>
        );
      },
    },
    {
      id: "scope",
      header: "Scope",
      size: 100,
      enableSorting: false,
      cell: ({ row }) => {
        const primaryRole = getPrimaryMemberRoleInScope(row.original, {
          level: scopeLevel,
          tenantCode,
          workspaceCode,
        });
        return primaryRole ? (
          <MemberScopeBadge scope={primaryRole.scope_type} />
        ) : (
          <span className="text-xs text-muted-foreground">-</span>
        );
      },
    },
    ...(canManage
      ? [
          {
            id: "actions",
            size: 40,
            enableSorting: false,
            cell: ({ row }: { row: { original: MemberWithRoles } }) => {
              const member = row.original;
              const scopedRoles = getMemberRolesInScope(member, {
                level: scopeLevel,
                tenantCode,
                workspaceCode,
              });
              const canRevokeAccess = scopedRoles.length > 0;
              return (
                <div className="flex items-center justify-end gap-1">
                  {rowActions.map((action) =>
                    action.kind === "change-role" ? (
                      <Tooltip key={action.kind}>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => setRoleEditorTarget(member)}
                          >
                            <ShieldCheck className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{action.label}</TooltipContent>
                      </Tooltip>
                    ) : canRevokeAccess ? (
                      <Tooltip key={action.kind}>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive"
                            onClick={() => setRevokeTarget(member)}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{action.label}</TooltipContent>
                      </Tooltip>
                    ) : null,
                  )}
                </div>
              );
            },
          } satisfies ColumnDef<MemberWithRoles>,
        ]
      : []),
  ];

  return (
    <>
      <div className="flex items-center justify-between mb-6">
        <div className="relative w-[280px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search members..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 h-9"
          />
        </div>
        {canManage && (
          <Button onClick={() => setInviteOpen(true)}>
            <UserPlus className="h-4 w-4 mr-2" />
            Invite Member
          </Button>
        )}
      </div>

      <DataTable
        columns={columns}
        data={members}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={Users}
            title="No members yet"
            description={`Invite team members to collaborate on ${scopeLabel(scopeLevel, tenantCode, workspaceCode)}.`}
            action={
              canManage ? (
                <Button onClick={() => setInviteOpen(true)}>
                  <UserPlus className="h-4 w-4 mr-2" />
                  Invite Member
                </Button>
              ) : undefined
            }
          />
        }
      />

      {canManage && (
        <InviteMemberForm
          open={inviteOpen}
          onOpenChange={setInviteOpen}
          onSubmit={handleInvite}
          allowedRoles={getAllowedMemberRolesForScope(scopeLevel)}
          scopeLabel={scopeLabel(scopeLevel, tenantCode, workspaceCode)}
        />
      )}

      {roleEditorTarget && canManage && (
        <RoleEditor
          open={!!roleEditorTarget}
          onOpenChange={(open) => {
            if (!open) setRoleEditorTarget(null);
          }}
          memberEmail={roleEditorTarget.email}
          onSubmit={(data) => handleChangeRole(roleEditorTarget.id, data)}
          scopeType={scopeLevel}
          allowedRoles={getAllowedMemberRolesForScope(scopeLevel)}
          scopeLabel={scopeLabel(scopeLevel, tenantCode, workspaceCode)}
          memberRoles={roleEditorTarget.roles}
        />
      )}

      <ConfirmDialog
        open={!!revokeTarget}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null);
        }}
        title={revokeDialogCopy.title}
        description={revokeDialogCopy.description}
        confirmLabel={revokeDialogCopy.confirmLabel}
        onConfirm={handleRevokeAccess}
        loading={revokeAccessMutation.isPending}
      />
    </>
  );
}

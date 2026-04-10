"use client";

import { useEffect, useMemo, useState } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { UserPlus, Users, ShieldPlus, Trash2, Search } from "lucide-react";
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
  useAddMemberRole,
  useRemoveMemberRole,
} from "@/hooks/use-members-mgmt";
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
    case "workspace":
      return roles.some((r) => r.role === "tenant_admin" || r.role === "workspace_admin");
    case "global":
    default:
      return false;
  }
}

function allowedRolesForScope(scopeLevel: ScopeLevel): Role[] {
  switch (scopeLevel) {
    case "workspace":
      return ["workspace_viewer", "workspace_editor", "workspace_admin"];
    case "global":
    default:
      return ["workspace_viewer", "workspace_editor", "workspace_admin", "tenant_admin", "superadmin"];
  }
}

function scopeLabel(scopeLevel: ScopeLevel, tenantCode?: string, workspaceCode?: string): string {
  switch (scopeLevel) {
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
  const addRoleMutation = useAddMemberRole();
  const removeRoleMutation = useRemoveMemberRole();

  const [inviteOpen, setInviteOpen] = useState(false);
  const [roleEditorTarget, setRoleEditorTarget] =
    useState<MemberWithRoles | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<{
    member: MemberWithRoles;
    role: MemberRoleDetail;
  } | null>(null);
  const [searchQuery, setSearchQuery] = useState("");

  useEffect(() => {
    if (error) toast.error("Failed to load members");
  }, [error]);

  const members = useMemo(() => {
    const allMembers = data?.pages.flatMap((p) => p.items) ?? [];
    const visibleMembers = allMembers.filter((member) => (member.roles?.length ?? 0) > 0);
    if (!searchQuery) return visibleMembers;
    return visibleMembers.filter(
      (m) =>
        m.email.toLowerCase().includes(searchQuery.toLowerCase()) ||
        m.display_name?.toLowerCase().includes(searchQuery.toLowerCase())
    );
  }, [data?.pages, searchQuery]);

  const handleInvite = async (formData: {
    email: string;
    display_name?: string;
    role: Role;
  }) => {
    await inviteMutation.mutateAsync(formData);
    toast.success("Member invited");
  };

  const handleAddRole = async (
    memberId: string,
    data: { role: Role; scope_type: ScopeLevel }
  ) => {
    await addRoleMutation.mutateAsync({
      memberId,
      data,
    });
    setRoleEditorTarget(null);
    toast.success("Role added");
  };

  const handleRemoveRole = async () => {
    if (!revokeTarget) return;
    await removeRoleMutation.mutateAsync({
      memberId: revokeTarget.member.id,
      roleId: revokeTarget.role.id,
    });
    setRevokeTarget(null);
    toast.success("Role removed");
  };

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
        const primaryRole = row.original.roles?.[0];
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
        const primaryRole = row.original.roles?.[0];
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
              return (
                <div className="flex items-center justify-end gap-1">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setRoleEditorTarget(member)}>
                        <ShieldPlus className="h-4 w-4" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>Add Role</TooltipContent>
                  </Tooltip>
                  {(member.roles ?? []).map((role) => (
                    <Tooltip key={role.id}>
                      <TooltipTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => setRevokeTarget({ member, role })}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Remove {role.role}</TooltipContent>
                    </Tooltip>
                  ))}
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
          allowedRoles={allowedRolesForScope(scopeLevel)}
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
          onSubmit={(data) => handleAddRole(roleEditorTarget.id, data)}
          scopeType={scopeLevel === "global" ? undefined : scopeLevel}
          allowedRoles={allowedRolesForScope(scopeLevel)}
          scopeLabel={scopeLabel(scopeLevel, tenantCode, workspaceCode)}
        />
      )}

      <ConfirmDialog
        open={!!revokeTarget}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null);
        }}
        title="Remove Role"
        description={`Remove the "${revokeTarget?.role.role ?? ""}" role from ${revokeTarget?.member.email ?? ""}? They may lose access to resources.`}
        confirmLabel="Remove Role"
        onConfirm={handleRemoveRole}
        loading={removeRoleMutation.isPending}
      />
    </>
  );
}

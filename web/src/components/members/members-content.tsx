"use client";

import { useState, useEffect } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { UserPlus, Users, MoreHorizontal, ShieldPlus, Trash2, Search } from "lucide-react";
import { toast } from "sonner";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { RoleBadge } from "./role-badge";
import { MemberScopeBadge } from "./scope-badge";
import { InviteMemberForm } from "./invite-member-form";
import { RoleEditor } from "./role-editor";
import {
  useMembers,
  useInviteMember,
  useAddMemberRole,
  useRemoveMemberRole,
} from "@/hooks/use-members-mgmt";
import { useScope } from "@/hooks/use-scope";
import type { MemberWithRoles, MemberRoleDetail } from "@/types/members-ext";

export function MembersContent() {
  const { level } = useScope();

  if (level !== "global") {
    return (
      <EmptyState
        icon={Users}
        title="Global scope required"
        description="Members management is only available at global scope. Switch to global scope to manage members."
      />
    );
  }

  return <MembersTable />;
}

function MembersTable() {
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

  const allMembers = data?.pages.flatMap((p) => p.items) ?? [];

  const members = searchQuery
    ? allMembers.filter(
        (m) =>
          m.email.toLowerCase().includes(searchQuery.toLowerCase()) ||
          m.display_name.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : allMembers;

  const handleInvite = async (formData: {
    email: string;
    display_name?: string;
  }) => {
    await inviteMutation.mutateAsync(formData);
    toast.success("Member invited");
  };

  const handleAddRole = async (
    memberId: string,
    roleData: { role: string; scope_type: string }
  ) => {
    await addRoleMutation.mutateAsync({
      memberId,
      data: roleData as Parameters<typeof addRoleMutation.mutateAsync>[0]["data"],
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
        const primaryRole = row.original.roles[0];
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
        const primaryRole = row.original.roles[0];
        return primaryRole ? (
          <MemberScopeBadge scope={primaryRole.scope_type} />
        ) : (
          <span className="text-xs text-muted-foreground">-</span>
        );
      },
    },
    {
      id: "actions",
      size: 40,
      enableSorting: false,
      cell: ({ row }) => {
        const member = row.original;
        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button className="p-1 rounded hover:bg-muted">
                <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                onClick={() => setRoleEditorTarget(member)}
              >
                <ShieldPlus className="h-4 w-4" />
                Add Role
              </DropdownMenuItem>
              {member.roles.map((role) => (
                <DropdownMenuItem
                  key={role.id}
                  variant="destructive"
                  onClick={() =>
                    setRevokeTarget({ member, role })
                  }
                >
                  <Trash2 className="h-4 w-4" />
                  Remove {role.role}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        );
      },
    },
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
        <Button onClick={() => setInviteOpen(true)}>
          <UserPlus className="h-4 w-4 mr-2" />
          Invite Member
        </Button>
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
            description="Invite team members to collaborate on this scope."
            action={
              <Button onClick={() => setInviteOpen(true)}>
                <UserPlus className="h-4 w-4 mr-2" />
                Invite Member
              </Button>
            }
          />
        }
      />

      <InviteMemberForm
        open={inviteOpen}
        onOpenChange={setInviteOpen}
        onSubmit={handleInvite}
      />

      {roleEditorTarget && (
        <RoleEditor
          open={!!roleEditorTarget}
          onOpenChange={(open) => {
            if (!open) setRoleEditorTarget(null);
          }}
          memberEmail={roleEditorTarget.email}
          onSubmit={(data) => handleAddRole(roleEditorTarget.id, data)}
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

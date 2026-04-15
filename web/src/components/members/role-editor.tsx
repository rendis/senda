"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useScope } from "@/hooks/use-scope";
import { getPrimaryMemberRoleInScope } from "@/hooks/members-mgmt-logic";
import {
  getRoleEditorState,
  getRoleEditorSubmitLabel,
} from "./role-editor.logic";
import type { MemberRoleDetail } from "@/types/members-ext";
import type { Role, ScopeLevel } from "@/types/api";

const ROLE_LABELS: Record<Role, string> = {
  superadmin: "Superadmin",
  tenant_admin: "Tenant Admin",
  workspace_admin: "Workspace Admin",
  workspace_editor: "Editor",
  workspace_viewer: "Viewer",
};

interface RoleEditorProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  memberEmail: string;
  memberRoles: MemberRoleDetail[];
  onSubmit: (data: { role: Role; scope_type: ScopeLevel }) => Promise<void>;
  scopeType: ScopeLevel;
  allowedRoles: Role[];
  scopeLabel?: string;
}

function roleLabel(role: Role): string {
  return ROLE_LABELS[role] ?? role;
}

export function RoleEditor({
  open,
  onOpenChange,
  memberEmail,
  memberRoles,
  onSubmit,
  scopeType,
  allowedRoles,
  scopeLabel = "this scope",
}: RoleEditorProps) {
  const routeScope = useScope();
  const currentRole = useMemo(
    () =>
      getPrimaryMemberRoleInScope(
        { roles: memberRoles },
        {
          level: scopeType,
          tenantCode: routeScope.tenantCode,
          workspaceCode: routeScope.workspaceCode,
        },
      )?.role,
    [memberRoles, routeScope.tenantCode, routeScope.workspaceCode, scopeType],
  );
  const [selectedRole, setSelectedRole] = useState<Role>(
    currentRole ?? allowedRoles[0] ?? "workspace_viewer",
  );
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    setSelectedRole(currentRole ?? allowedRoles[0] ?? "workspace_viewer");
  }, [allowedRoles, currentRole, open]);

  const editorState = useMemo(
    () =>
      getRoleEditorState({
        allowedRoles,
        currentRole,
        selectedRole,
      }),
    [allowedRoles, currentRole, selectedRole],
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (editorState.submitDisabled) {
      return;
    }

    setLoading(true);
    try {
      await onSubmit({ role: selectedRole, scope_type: scopeType });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Change Role</DialogTitle>
            <DialogDescription>
              Set the single local role for {memberEmail} in {scopeLabel}.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>Role</Label>
              <Select
                value={selectedRole}
                onValueChange={(value) => setSelectedRole(value as Role)}
                disabled={loading || editorState.selectDisabled}
              >
                <SelectTrigger className="w-full min-h-10">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {editorState.roleOptions.map((item) => (
                    <SelectItem key={item} value={item}>
                      {roleLabel(item)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {editorState.helperText}
              </p>
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={loading || editorState.submitDisabled}>
              {loading ? "..." : getRoleEditorSubmitLabel(currentRole)}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

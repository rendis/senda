"use client";

import { useState } from "react";
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
import type { Role, ScopeLevel } from "@/types/api";

const ROLE_LABELS: Record<Role, string> = {
  superadmin: "Superadmin",
  tenant_admin: "Tenant Admin",
  workspace_admin: "Workspace Admin",
  workspace_editor: "Editor",
  workspace_viewer: "Viewer",
};

const ALL_ROLES: Role[] = [
  "workspace_viewer",
  "workspace_editor",
  "workspace_admin",
  "tenant_admin",
  "superadmin",
];

const ALL_SCOPES: ScopeLevel[] = ["global", "tenant", "workspace"];

interface RoleEditorProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  memberEmail: string;
  onSubmit: (data: { role: Role; scope_type: ScopeLevel }) => Promise<void>;
  scopeType?: ScopeLevel;
  allowedRoles?: Role[];
  scopeLabel?: string;
}

export function RoleEditor({
  open,
  onOpenChange,
  memberEmail,
  onSubmit,
  scopeType,
  allowedRoles,
  scopeLabel = "this scope",
}: RoleEditorProps) {
  const initialRole = allowedRoles?.[0] ?? "workspace_viewer";
  const [role, setRole] = useState<Role>(initialRole);
  const [selectedScopeType, setSelectedScopeType] = useState<ScopeLevel>(scopeType ?? "workspace");
  const [loading, setLoading] = useState(false);

  const roleOptions = (allowedRoles ?? ALL_ROLES).map((value) => ({
    value,
    label: ROLE_LABELS[value],
  }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await onSubmit({ role, scope_type: scopeType ?? selectedScopeType });
      onOpenChange(false);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Add Role</DialogTitle>
            <DialogDescription>
              Add a role for {memberEmail} in {scopeLabel}.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>Role</Label>
              <Select
                value={role}
                onValueChange={(v) => setRole(v as Role)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {roleOptions.map((r) => (
                    <SelectItem key={r.value} value={r.value}>
                      {r.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {scopeType == null && (
              <div className="space-y-2">
                <Label>Scope</Label>
                <Select
                  value={selectedScopeType}
                  onValueChange={(v) => setSelectedScopeType(v as ScopeLevel)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ALL_SCOPES.map((s) => (
                      <SelectItem key={s} value={s}>
                        {s.charAt(0).toUpperCase() + s.slice(1)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
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
            <Button type="submit" disabled={loading}>
              {loading ? "..." : "Add Role"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

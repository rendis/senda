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

interface RoleEditorProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  memberEmail: string;
  onSubmit: (data: { role: Role; scope_type: ScopeLevel }) => Promise<void>;
}

const ROLES: { value: Role; label: string }[] = [
  { value: "workspace_viewer", label: "Viewer" },
  { value: "workspace_editor", label: "Editor" },
  { value: "workspace_admin", label: "Workspace Admin" },
  { value: "tenant_admin", label: "Tenant Admin" },
  { value: "superadmin", label: "Superadmin" },
];

const SCOPES: { value: ScopeLevel; label: string }[] = [
  { value: "global", label: "Global" },
  { value: "tenant", label: "Tenant" },
  { value: "workspace", label: "Workspace" },
];

export function RoleEditor({
  open,
  onOpenChange,
  memberEmail,
  onSubmit,
}: RoleEditorProps) {
  const [role, setRole] = useState<Role>("workspace_viewer");
  const [scopeType, setScopeType] = useState<ScopeLevel>("workspace");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await onSubmit({ role, scope_type: scopeType });
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
              Add a role for {memberEmail}.
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
                  {ROLES.map((r) => (
                    <SelectItem key={r.value} value={r.value}>
                      {r.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Scope</Label>
              <Select
                value={scopeType}
                onValueChange={(v) => setScopeType(v as ScopeLevel)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SCOPES.map((s) => (
                    <SelectItem key={s.value} value={s.value}>
                      {s.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
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
            <Button type="submit" disabled={loading}>
              {loading ? "..." : "Add Role"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

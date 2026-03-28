import type { Role } from "@/types/api";

const ROLE_COLORS: Record<Role, string> = {
  superadmin: "text-scope-global",
  tenant_admin: "text-scope-system",
  workspace_admin: "text-scope-workspace",
  workspace_editor: "text-scope-workspace",
  workspace_viewer: "text-muted-foreground",
};

const ROLE_LABELS: Record<Role, string> = {
  superadmin: "superadmin",
  tenant_admin: "tenant_admin",
  workspace_admin: "ws_admin",
  workspace_editor: "ws_editor",
  workspace_viewer: "ws_viewer",
};

interface RoleBadgeProps {
  role: Role;
}

export function RoleBadge({ role }: RoleBadgeProps) {
  return (
    <span className={`font-mono text-xs font-medium ${ROLE_COLORS[role]}`}>
      {ROLE_LABELS[role]}
    </span>
  );
}

import type { Role } from "@/types/api";

const ROLE_COLORS: Record<Role, string> = {
  superadmin: "text-blue-500",
  tenant_admin: "text-purple-500",
  workspace_admin: "text-teal-600",
  workspace_editor: "text-teal-500",
  workspace_viewer: "text-gray-500",
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

import { Globe, Settings, Box } from "lucide-react";
import type { ScopeLevel } from "@/types/api";

const SCOPE_CONFIG: Record<
  ScopeLevel,
  { icon: typeof Globe; label: string; color: string; bg: string }
> = {
  global: {
    icon: Globe,
    label: "Global",
    color: "text-scope-global",
    bg: "bg-scope-global-bg",
  },
  tenant: {
    icon: Settings,
    label: "Tenant",
    color: "text-scope-system",
    bg: "bg-scope-system-bg",
  },
  workspace: {
    icon: Box,
    label: "Workspace",
    color: "text-scope-workspace",
    bg: "bg-scope-workspace-bg",
  },
};

interface ScopeBadgeProps {
  scope: ScopeLevel;
}

export function MemberScopeBadge({ scope }: ScopeBadgeProps) {
  const config = SCOPE_CONFIG[scope];
  const Icon = config.icon;

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 h-6 font-mono text-[11px] font-medium ${config.bg} ${config.color}`}
    >
      <Icon className="h-3 w-3" />
      {config.label}
    </span>
  );
}

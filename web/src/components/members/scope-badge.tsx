import { Globe, Settings, Box } from "lucide-react";
import type { ScopeLevel } from "@/types/api";

const SCOPE_CONFIG: Record<
  ScopeLevel,
  { icon: typeof Globe; label: string; color: string; bg: string }
> = {
  global: {
    icon: Globe,
    label: "Global",
    color: "text-blue-500",
    bg: "bg-blue-50",
  },
  tenant: {
    icon: Settings,
    label: "Tenant",
    color: "text-purple-500",
    bg: "bg-purple-50",
  },
  workspace: {
    icon: Box,
    label: "Workspace",
    color: "text-teal-500",
    bg: "bg-teal-50",
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

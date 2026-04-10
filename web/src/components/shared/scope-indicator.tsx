import { cn } from "@/lib/utils";
import { Globe, Settings, Box } from "lucide-react";
import type { ScopeLevel } from "@/types/api";
import { SYSTEM_WORKSPACE_LABEL } from "@/lib/system-workspace-display";

const scopeConfig: Record<
  ScopeLevel | "system",
  { label: string; icon: typeof Globe; color: string; bgColor: string }
> = {
  global: {
    label: "Global",
    icon: Globe,
    color: "text-scope-global",
    bgColor: "bg-scope-global-bg",
  },
  system: {
    label: SYSTEM_WORKSPACE_LABEL,
    icon: Settings,
    color: "text-scope-system",
    bgColor: "bg-scope-system-bg",
  },
  tenant: {
    label: "Tenant",
    icon: Settings,
    color: "text-scope-system",
    bgColor: "bg-scope-system-bg",
  },
  workspace: {
    label: "Workspace",
    icon: Box,
    color: "text-scope-workspace",
    bgColor: "bg-scope-workspace-bg",
  },
};

interface ScopeIndicatorProps {
  scope: ScopeLevel | "system";
  label?: string;
  className?: string;
}

export function ScopeIndicator({ scope, label, className }: ScopeIndicatorProps) {
  const config = scopeConfig[scope] ?? scopeConfig.global;
  const Icon = config.icon;

  return (
    <span
      className={cn(
        "inline-flex items-center justify-center gap-1.5 rounded-full px-2.5 h-6 font-mono text-[11px] font-medium",
        config.bgColor,
        config.color,
        className
      )}
    >
      <Icon className="h-3 w-3" />
      {label ?? config.label}
    </span>
  );
}

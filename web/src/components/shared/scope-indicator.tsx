import { cn } from "@/lib/utils";
import { Globe, Settings, Box } from "lucide-react";
import type { ScopeLevel } from "@/types/api";

const scopeConfig: Record<
  ScopeLevel | "system",
  { label: string; icon: typeof Globe; color: string; bgColor: string }
> = {
  global: {
    label: "Global",
    icon: Globe,
    color: "text-scope-global",
    bgColor: "bg-blue-50",
  },
  system: {
    label: "_system",
    icon: Settings,
    color: "text-scope-system",
    bgColor: "bg-violet-50",
  },
  tenant: {
    label: "Tenant",
    icon: Settings,
    color: "text-scope-system",
    bgColor: "bg-violet-50",
  },
  workspace: {
    label: "Workspace",
    icon: Box,
    color: "text-scope-workspace",
    bgColor: "bg-primary-light",
  },
};

interface ScopeIndicatorProps {
  scope: ScopeLevel | "system";
  label?: string;
  className?: string;
}

export function ScopeIndicator({ scope, label, className }: ScopeIndicatorProps) {
  const config = scopeConfig[scope];
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

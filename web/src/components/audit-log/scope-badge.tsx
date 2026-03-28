import { Globe, Box, Settings } from "lucide-react";
import { cn } from "@/lib/utils";

const scopeConfig: Record<
  string,
  { icon: typeof Globe; bg: string; text: string; label: string }
> = {
  global: {
    icon: Globe,
    bg: "bg-scope-global-bg",
    text: "text-scope-global",
    label: "Global",
  },
  tenant: {
    icon: Settings,
    bg: "bg-scope-system-bg",
    text: "text-scope-system",
    label: "Tenant",
  },
  workspace: {
    icon: Box,
    bg: "bg-scope-workspace-bg",
    text: "text-scope-workspace",
    label: "Workspace",
  },
};

const defaultScope = {
  icon: Globe,
  bg: "bg-muted",
  text: "text-muted-foreground",
  label: "Unknown",
};

interface ScopeBadgeProps {
  scope: string;
  className?: string;
}

export function ScopeBadge({ scope, className }: ScopeBadgeProps) {
  const config = scopeConfig[scope] ?? defaultScope;
  const Icon = config.icon;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 h-6 font-mono text-[11px] font-medium",
        config.bg,
        config.text,
        className
      )}
    >
      <Icon className="h-3 w-3" />
      {config.label}
    </span>
  );
}

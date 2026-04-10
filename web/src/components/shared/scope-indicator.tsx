"use client";

import { useTranslations } from "next-intl";
import { Globe, Settings, Box } from "lucide-react";

import { cn } from "@/lib/utils";
import type { ScopeLevel } from "@/types/api";

const scopeConfig: Record<
  ScopeLevel | "system",
  {
    labelKey: "global" | "system" | "tenant" | "workspace";
    icon: typeof Globe;
    color: string;
    bgColor: string;
  }
> = {
  global: {
    labelKey: "global",
    icon: Globe,
    color: "text-scope-global",
    bgColor: "bg-scope-global-bg",
  },
  system: {
    labelKey: "system",
    icon: Settings,
    color: "text-scope-system",
    bgColor: "bg-scope-system-bg",
  },
  tenant: {
    labelKey: "tenant",
    icon: Settings,
    color: "text-scope-system",
    bgColor: "bg-scope-system-bg",
  },
  workspace: {
    labelKey: "workspace",
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

export function ScopeIndicator({
  scope,
  label,
  className,
}: ScopeIndicatorProps) {
  const t = useTranslations("scopeIndicator");
  const config = scopeConfig[scope] ?? scopeConfig.global;
  const Icon = config.icon;

  return (
    <span
      className={cn(
        "inline-flex items-center justify-center gap-1.5 rounded-full px-2.5 h-6 font-mono text-[11px] font-medium",
        config.bgColor,
        config.color,
        className,
      )}
    >
      <Icon className="h-3 w-3" />
      {label ?? t(config.labelKey)}
    </span>
  );
}

"use client";

import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { ResourceStateBadge } from "@/lib/workspace-resource-policies";

const badgeClassByKey: Record<ResourceStateBadge, string> = {
  local:
    "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  defaultSystem:
    "border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-300",
  forkedFromDefault:
    "border-violet-500/30 bg-violet-500/10 text-violet-700 dark:text-violet-300",
  readOnly:
    "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  workspace:
    "border-slate-500/30 bg-slate-500/10 text-slate-700 dark:text-slate-300",
  global:
    "border-zinc-500/30 bg-zinc-500/10 text-zinc-700 dark:text-zinc-300",
};

export function ResourceStateBadges({
  badges,
  className,
}: {
  badges: ResourceStateBadge[];
  className?: string;
}) {
  const t = useTranslations("resourceState");

  if (!badges.length) {
    return null;
  }

  return (
    <div className={cn("flex flex-wrap items-center gap-2", className)}>
      {badges.map((badge) => (
        <Badge
          key={badge}
          variant="outline"
          className={cn("text-[11px]", badgeClassByKey[badge])}
        >
          {t(`badges.${badge}`)}
        </Badge>
      ))}
    </div>
  );
}

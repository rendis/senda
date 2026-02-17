"use client";

import { cn } from "@/lib/utils";
import { formatRelativeTime } from "@/lib/utils";
import type { DashboardActivityItem } from "@/types/api";

interface ActivityFeedProps {
  items: DashboardActivityItem[];
  auditHref?: string;
}

function getActivityColor(action: string): string {
  if (action.includes("delete") || action.includes("bounce") || action.includes("fail")) {
    return "bg-[#EF4444]";
  }
  if (action.includes("create") || action.includes("deliver") || action.includes("send")) {
    return "bg-[#22C55E]";
  }
  return "bg-[#3B82F6]";
}

function formatAction(action: string, entityType: string): string {
  const verb = action.replace(/_/g, " ");
  return `${verb} ${entityType}`;
}

export function ActivityFeed({ items, auditHref }: ActivityFeedProps) {
  return (
    <div className="w-[360px] shrink-0 rounded-lg border border-border bg-card p-5 flex flex-col gap-3 overflow-hidden">
      <div className="flex items-center justify-between">
        <span className="text-sm font-semibold text-foreground">
          Recent Activity
        </span>
        {auditHref && (
          <a
            href={auditHref}
            className="text-xs font-medium text-primary hover:underline"
          >
            View all
          </a>
        )}
      </div>
      <div className="flex flex-col overflow-y-auto">
        {items.map((item, i) => (
          <div
            key={item.id}
            className={cn(
              "flex items-center gap-3 py-2.5",
              i < items.length - 1 && "border-b border-border"
            )}
          >
            <div
              className={cn(
                "w-[3px] h-8 rounded-sm shrink-0",
                getActivityColor(item.action)
              )}
            />
            <div className="flex flex-col gap-0.5 min-w-0">
              <span className="text-xs font-medium text-foreground truncate">
                {formatAction(item.action, item.entity_type)}
              </span>
              <span className="text-[11px] font-mono text-muted-foreground">
                {formatRelativeTime(item.created_at)}
              </span>
            </div>
          </div>
        ))}
        {items.length === 0 && (
          <p className="text-xs font-mono text-muted-foreground py-4 text-center">
            No recent activity
          </p>
        )}
      </div>
    </div>
  );
}

export function ActivityFeedSkeleton() {
  return (
    <div className="w-[360px] shrink-0 rounded-lg border border-border bg-card p-5 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="h-4 w-28 rounded bg-accent animate-pulse" />
        <div className="h-3 w-12 rounded bg-accent animate-pulse" />
      </div>
      <div className="flex flex-col gap-0">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3 py-2.5 border-b border-border last:border-0">
            <div className="w-[3px] h-8 rounded-sm bg-accent animate-pulse shrink-0" />
            <div className="flex flex-col gap-1">
              <div className="h-3 w-32 rounded bg-accent animate-pulse" />
              <div className="h-2.5 w-16 rounded bg-accent animate-pulse" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface MetricCardProps {
  label: string;
  value: string;
  icon: LucideIcon;
  change?: string;
  period?: string;
  className?: string;
}

export function MetricCard({
  label,
  value,
  icon: Icon,
  change,
  period,
  className,
}: MetricCardProps) {
  const isPositive = change?.startsWith("+");
  const isNegative = change?.startsWith("-");

  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-card p-5 flex flex-col gap-3",
        className
      )}
    >
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium font-mono text-foreground/70">
          {label}
        </span>
        <Icon className="h-4 w-4 text-foreground/70" />
      </div>
      <p
        className="text-[28px] font-semibold text-foreground"
        style={{ letterSpacing: "-2px" }}
      >
        {value}
      </p>
      {(change || period) && (
        <div className="flex items-center gap-1">
          {change && (
            <span
              className={cn(
                "text-xs font-medium font-mono",
                isPositive && "text-status-delivered",
                isNegative && "text-status-bounced",
                !isPositive && !isNegative && "text-foreground/70"
              )}
            >
              {change}
            </span>
          )}
          {period && (
            <span className="text-xs font-mono text-foreground/70">
              {period}
            </span>
          )}
        </div>
      )}
    </div>
  );
}

export function MetricCardSkeleton() {
  return (
    <div className="rounded-lg border border-border bg-card p-5 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="h-3 w-20 rounded bg-accent animate-pulse" />
        <div className="h-4 w-4 rounded bg-accent animate-pulse" />
      </div>
      <div className="h-8 w-24 rounded bg-accent animate-pulse" />
      <div className="h-3 w-28 rounded bg-accent animate-pulse" />
    </div>
  );
}

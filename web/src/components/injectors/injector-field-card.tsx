"use client";

import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

interface InjectorFieldCardProps {
  title: string;
  typeLabel?: string;
  actions?: ReactNode;
  children: ReactNode;
  description?: string;
  testId?: string;
  headerTestId?: string;
  className?: string;
}

export function InjectorFieldCard({
  title,
  typeLabel,
  actions,
  children,
  description,
  testId,
  headerTestId,
  className,
}: InjectorFieldCardProps) {
  return (
    <section
      data-testid={testId}
      className={cn(
        "overflow-hidden rounded-xl border border-border/70 bg-card/95",
        className
      )}
    >
      <div
        data-testid={headerTestId}
        className="flex items-start justify-between gap-3 border-b border-border/60 bg-muted/20 px-4 py-3"
      >
        <div className="min-w-0 space-y-1">
          <div className="flex min-w-0 items-center gap-2">
            <p className="truncate text-sm font-medium text-foreground">{title}</p>
            {typeLabel ? (
              <Badge
                variant="outline"
                className="border-border/70 bg-background/80 px-2 py-0 text-[11px] font-medium text-muted-foreground"
              >
                {typeLabel}
              </Badge>
            ) : null}
          </div>
          {description ? (
            <p className="line-clamp-1 text-xs text-muted-foreground">
              {description}
            </p>
          ) : null}
        </div>

        {actions ? (
          <div className="flex shrink-0 items-center gap-1">{actions}</div>
        ) : null}
      </div>

      <div className="flex flex-col gap-3 p-4">{children}</div>
    </section>
  );
}

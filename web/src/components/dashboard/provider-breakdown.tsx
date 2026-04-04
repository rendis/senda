"use client";

import type { DashboardAdapterTotals } from "@/types/api";

interface ProviderBreakdownProps {
  adapters: DashboardAdapterTotals[];
}

function formatNumber(n: number): string {
  return n.toLocaleString("en-US");
}

function deliveryRate(t: DashboardAdapterTotals["totals"]): string {
  if (t.sent === 0) return "0%";
  return `${((t.delivered / t.sent) * 100).toFixed(1)}%`;
}

const TRACKING_TYPES = new Set(["ses"]);

export function ProviderBreakdown({ adapters }: ProviderBreakdownProps) {
  if (!adapters || adapters.length === 0) return null;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-foreground">By Provider</h3>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        {adapters.map((a) => {
          const hasTracking = TRACKING_TYPES.has(a.adapter_type);
          return (
            <div
              key={`${a.adapter_id}:${a.sender_identity_id ?? a.from_email}`}
              className="rounded-lg border bg-card p-4 flex flex-col gap-2"
            >
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-foreground">
                  {a.adapter_name}
                </span>
                <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-mono uppercase text-foreground/70">
                  {a.adapter_type}
                </span>
                {a.from_email && (
                  <span className="rounded bg-scope-system-bg px-1.5 py-0.5 text-[10px] font-mono text-scope-system">
                    {a.from_email}
                  </span>
                )}
                {hasTracking ? (
                  <span className="rounded bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-mono text-emerald-700">
                    tracking
                  </span>
                ) : (
                  <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-mono text-amber-700">
                    send-only
                  </span>
                )}
              </div>
              <div className="grid grid-cols-4 gap-2 text-xs">
                <div>
                  <span className="text-foreground/70">Sent</span>
                  <p className="font-mono font-medium">
                    {formatNumber(a.totals.sent)}
                  </p>
                </div>
                <div>
                  <span className="text-foreground/70">Delivered</span>
                  <p className="font-mono font-medium">
                    {hasTracking ? formatNumber(a.totals.delivered) : "-"}
                  </p>
                </div>
                <div>
                  <span className="text-foreground/70">Bounced</span>
                  <p className="font-mono font-medium">
                    {hasTracking ? formatNumber(a.totals.bounced) : "-"}
                  </p>
                </div>
                <div>
                  <span className="text-foreground/70">Delivery</span>
                  <p className="font-mono font-medium">
                    {hasTracking ? deliveryRate(a.totals) : "-"}
                  </p>
                </div>
              </div>
              {!hasTracking && (
                <p className="mt-1 text-[10px] text-foreground/70">
                  This provider does not support delivery/bounce tracking.
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

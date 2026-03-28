"use client";

import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  ResponsiveContainer,
  Tooltip,
} from "recharts";
import type { DashboardTimeSeriesPoint } from "@/types/api";
import type { DateRange } from "@/hooks/use-dashboard-stats";

interface EmailBarChartProps {
  data: DashboardTimeSeriesPoint[];
  range: DateRange;
}

function formatDayLabel(dateStr: string): string {
  const date = new Date(dateStr + "T00:00:00");
  return date.toLocaleDateString("en-US", { weekday: "short" });
}

export function EmailBarChart({ data, range }: EmailBarChartProps) {
  const chartData = data.map((d) => ({
    ...d,
    label: formatDayLabel(d.date),
  }));

  const rangeLabel = range === "7d" ? "Last 7 days" : "Last 30 days";

  return (
    <div className="flex-1 rounded-lg border border-border bg-card p-5 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-semibold text-foreground">
          Emails Sent
        </span>
        <span className="text-[11px] font-mono text-foreground/70">
          {rangeLabel}
        </span>
      </div>
      <div className="flex-1 min-h-0">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={chartData} margin={{ top: 0, right: 0, left: -20, bottom: 0 }}>
            <XAxis
              dataKey="label"
              axisLine={false}
              tickLine={false}
              tick={{
                fontSize: 10,
                fontFamily: "var(--font-ibm-plex-mono)",
                fill: "var(--muted-foreground)",
              }}
            />
            <YAxis
              axisLine={false}
              tickLine={false}
              tick={{
                fontSize: 10,
                fontFamily: "var(--font-ibm-plex-mono)",
                fill: "var(--muted-foreground)",
              }}
              width={40}
            />
            <Tooltip
              contentStyle={{
                fontSize: 12,
                fontFamily: "var(--font-ibm-plex-mono)",
                borderRadius: 8,
                border: "1px solid #E2E8F0",
              }}
            />
            <Bar
              dataKey="sent"
              fill="var(--primary)"
              radius={[4, 4, 0, 0]}
              maxBarSize={32}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

export function EmailBarChartSkeleton() {
  return (
    <div className="flex-1 rounded-lg border border-border bg-card p-5 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div className="h-4 w-24 rounded bg-accent animate-pulse" />
        <div className="h-3 w-16 rounded bg-accent animate-pulse" />
      </div>
      <div className="flex-1 flex items-end gap-2 pt-4">
        {[45, 70, 35, 80, 55, 65, 50].map((h, i) => (
          <div
            key={i}
            className="flex-1 rounded-t bg-accent animate-pulse"
            style={{ height: `${h}%` }}
          />
        ))}
      </div>
    </div>
  );
}

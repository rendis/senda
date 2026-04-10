"use client";

import { useState, useEffect } from "react";
import { useMinimumLoading } from "@/hooks/use-minimum-loading";
import { Mail, Send, CheckCircle, AlertTriangle, AlertCircle, RefreshCw } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import {
  SYSTEM_WORKSPACE_SCOPE_LABEL,
  getWorkspaceDisplayCode,
} from "@/lib/system-workspace-display";
import { useDashboardStats, type DateRange } from "@/hooks/use-dashboard-stats";
import { MetricCard, MetricCardSkeleton } from "@/components/shared/metric-card";
import { DateRangePicker } from "@/components/dashboard/date-range-picker";
import { EmailBarChart, EmailBarChartSkeleton } from "@/components/dashboard/email-bar-chart";
import { ActivityFeed, ActivityFeedSkeleton } from "@/components/dashboard/activity-feed";
import { RecentEmailsTable, RecentEmailsTableSkeleton } from "@/components/dashboard/recent-emails-table";
import { ProviderBreakdown } from "@/components/dashboard/provider-breakdown";
import { EmptyState } from "@/components/shared/empty-state";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

function formatNumber(n: number): string {
  return n.toLocaleString("en-US");
}

function formatRate(rate: number): string {
  return `${(rate * 100).toFixed(1)}%`;
}

export function DashboardContent() {
  const [range, setRange] = useState<DateRange>("7d");
  const scope = useScope();
  const scopedPath = useScopedPath();
  const { data, isLoading: rawLoading, error, refetch } = useDashboardStats(scopedPath, range);
  const isLoading = useMinimumLoading(rawLoading);

  useEffect(() => {
    if (error) toast.error("Failed to load dashboard stats");
  }, [error]);

  const scopeLabel =
    scope.level === "workspace"
      ? scope.workspaceCode === SYSTEM_WORKSPACE_CODE
        ? SYSTEM_WORKSPACE_SCOPE_LABEL
        : `Workspace: ${getWorkspaceDisplayCode({ code: scope.workspaceCode })}`
      : "All tenants and workspaces";

  const rangeLabel = range === "7d" ? "vs last 7 days" : "vs last 30 days";

  // Build "View all" hrefs based on scope
  const auditHref =
    scope.level === "workspace"
      ? `/t/${scope.tenantCode}/w/${scope.workspaceCode}/audit`
      : undefined;

  const emailsHref =
    scope.level === "workspace"
      ? `/t/${scope.tenantCode}/w/${scope.workspaceCode}/emails`
      : undefined;

  // Loading state
  if (isLoading) {
    return (
      <div className="flex flex-col gap-7">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-semibold" style={{ letterSpacing: "-1px" }}>
              Overview
            </h2>
            <p className="mt-1 text-xs font-mono text-foreground/70">
              {scopeLabel}
            </p>
          </div>
          <DateRangePicker value={range} onChange={setRange} />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <MetricCardSkeleton key={i} />
          ))}
        </div>
        <div className="flex gap-6 h-60">
          <EmailBarChartSkeleton />
          <ActivityFeedSkeleton />
        </div>
        <RecentEmailsTableSkeleton />
      </div>
    );
  }

  // Empty state (no data at all)
  if (data && data.totals.sent === 0 && data.recent_emails.length === 0) {
    return (
      <div className="flex flex-col gap-7">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-semibold" style={{ letterSpacing: "-1px" }}>
              Overview
            </h2>
            <p className="mt-1 text-xs font-mono text-foreground/70">
              {scopeLabel}
            </p>
          </div>
          <DateRangePicker value={range} onChange={setRange} />
        </div>
        <EmptyState
          icon={Mail}
          title="No emails sent yet"
          description="Start sending emails to see your dashboard metrics, delivery rates, and activity here."
        />
      </div>
    );
  }

  // Error state (no cached data)
  if (error && !data) {
    return (
      <div className="flex flex-col gap-7">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-semibold" style={{ letterSpacing: "-1px" }}>
              Overview
            </h2>
            <p className="mt-1 text-xs font-mono text-foreground/70">
              {scopeLabel}
            </p>
          </div>
          <DateRangePicker value={range} onChange={setRange} />
        </div>
        <EmptyState
          icon={RefreshCw}
          title="Failed to load dashboard"
          description="Something went wrong while loading your dashboard stats. Please try again."
          action={
            <Button variant="outline" onClick={() => refetch()}>
              Retry
            </Button>
          }
        />
      </div>
    );
  }

  // Data loaded
  if (!data) return null;

  return (
    <div className="flex flex-col gap-7 animate-in fade-in duration-300">
      {/* Content Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2
            className="text-2xl font-semibold"
            style={{ letterSpacing: "-1px" }}
          >
            Overview
          </h2>
          <p className="mt-1 text-xs font-mono text-foreground/70">
            {scopeLabel}
          </p>
        </div>
        <DateRangePicker value={range} onChange={setRange} />
      </div>

      {/* Metrics Row */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          label="Emails Sent"
          value={formatNumber(data.totals.sent)}
          icon={Send}
          period={rangeLabel}
        />
        <MetricCard
          label="Delivery Rate"
          value={formatRate(data.rates.delivery_rate)}
          icon={CheckCircle}
          period={rangeLabel}
        />
        <MetricCard
          label="Bounce Rate"
          value={formatRate(data.rates.bounce_rate)}
          icon={AlertTriangle}
          period={rangeLabel}
        />
        <MetricCard
          label="Complaint Rate"
          value={formatRate(data.rates.complaint_rate)}
          icon={AlertCircle}
          period={rangeLabel}
        />
      </div>

      {/* Mid Row: Chart + Activity */}
      <div className="flex gap-6 h-60">
        <EmailBarChart data={data.time_series} range={range} />
        <ActivityFeed items={data.recent_activity} auditHref={auditHref} />
      </div>

      {/* Provider Breakdown */}
      <ProviderBreakdown adapters={data.by_adapter} />

      {/* Recent Emails Table */}
      <RecentEmailsTable
        emails={data.recent_emails}
        emailsHref={emailsHref}
      />
    </div>
  );
}

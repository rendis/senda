"use client";

import { StatusBadge } from "@/components/shared/status-badge";
import { formatDate } from "@/lib/utils";
import type { DashboardRecentEmail } from "@/types/api";

interface RecentEmailsTableProps {
  emails: DashboardRecentEmail[];
  emailsHref?: string;
}

export function RecentEmailsTable({ emails, emailsHref }: RecentEmailsTableProps) {
  return (
    <div className="rounded-lg border border-border bg-card flex flex-col overflow-hidden">
      <div className="flex items-center justify-between px-5 py-4 border-b border-border">
        <span className="text-sm font-semibold text-foreground">
          Recent Emails
        </span>
        {emailsHref && (
          <a
            href={emailsHref}
            className="text-xs font-medium text-primary hover:underline"
          >
            View all
          </a>
        )}
      </div>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="h-9 bg-surface">
              <th className="text-left px-5 text-[11px] font-semibold font-mono text-muted-foreground uppercase tracking-wider">
                Recipient
              </th>
              <th className="text-left px-5 text-[11px] font-semibold font-mono text-muted-foreground uppercase tracking-wider w-[140px]">
                Template
              </th>
              <th className="text-left px-5 text-[11px] font-semibold font-mono text-muted-foreground uppercase tracking-wider w-[110px]">
                Status
              </th>
              <th className="text-left px-5 text-[11px] font-semibold font-mono text-muted-foreground uppercase tracking-wider w-[150px]">
                Date
              </th>
            </tr>
          </thead>
          <tbody>
            {emails.map((email) => (
              <tr
                key={email.id}
                className="h-11 border-b border-border last:border-0"
              >
                <td className="px-5 text-[13px] font-mono text-foreground truncate max-w-0">
                  {email.recipient_email}
                </td>
                <td className="px-5 text-[13px] font-mono text-muted-foreground w-[140px]">
                  {email.template_type_slug || "\u2014"}
                </td>
                <td className="px-5 w-[110px]">
                  <StatusBadge status={email.status} />
                </td>
                <td className="px-5 text-xs font-mono text-muted-foreground w-[150px]">
                  {formatDate(email.created_at)}
                </td>
              </tr>
            ))}
            {emails.length === 0 && (
              <tr>
                <td
                  colSpan={4}
                  className="px-5 py-8 text-center text-xs font-mono text-muted-foreground"
                >
                  No emails sent yet
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export function RecentEmailsTableSkeleton() {
  return (
    <div className="rounded-lg border border-border bg-card flex flex-col overflow-hidden">
      <div className="flex items-center justify-between px-5 py-4 border-b border-border">
        <div className="h-4 w-28 rounded bg-accent animate-pulse" />
        <div className="h-3 w-12 rounded bg-accent animate-pulse" />
      </div>
      <div className="h-9 bg-surface" />
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className="h-11 px-5 flex items-center gap-8 border-b border-border last:border-0"
        >
          <div className="h-3 flex-1 rounded bg-accent animate-pulse" />
          <div className="h-3 w-[100px] rounded bg-accent animate-pulse" />
          <div className="h-6 w-[80px] rounded-full bg-accent animate-pulse" />
          <div className="h-3 w-[110px] rounded bg-accent animate-pulse" />
        </div>
      ))}
    </div>
  );
}

import { cn } from "@/lib/utils";
import type { EmailStatus, TemplateVersionStatus, DomainStatus, WebhookStatus } from "@/types/api";

type Status = EmailStatus | TemplateVersionStatus | DomainStatus | WebhookStatus | "revoked";

const statusConfig: Record<
  Status,
  { label: string; dotColor: string; bgColor: string; textColor: string }
> = {
  queued: {
    label: "Queued",
    dotColor: "bg-status-queued",
    bgColor: "bg-status-queued-bg",
    textColor: "text-status-queued",
  },
  processing: {
    label: "Processing",
    dotColor: "bg-status-queued",
    bgColor: "bg-status-queued-bg",
    textColor: "text-status-queued",
  },
  sent: {
    label: "Sent",
    dotColor: "bg-status-sent",
    bgColor: "bg-status-sent-bg",
    textColor: "text-status-sent",
  },
  delivered: {
    label: "Delivered",
    dotColor: "bg-status-delivered",
    bgColor: "bg-status-delivered-bg",
    textColor: "text-status-delivered",
  },
  opened: {
    label: "Opened",
    dotColor: "bg-status-delivered",
    bgColor: "bg-status-delivered-bg",
    textColor: "text-status-delivered",
  },
  bounced: {
    label: "Bounced",
    dotColor: "bg-status-bounced",
    bgColor: "bg-status-bounced-bg",
    textColor: "text-status-bounced",
  },
  complained: {
    label: "Complained",
    dotColor: "bg-status-complained",
    bgColor: "bg-status-complained-bg",
    textColor: "text-status-complained",
  },
  failed: {
    label: "Failed",
    dotColor: "bg-status-bounced",
    bgColor: "bg-status-bounced-bg",
    textColor: "text-status-bounced",
  },
  suppressed: {
    label: "Suppressed",
    dotColor: "bg-status-queued",
    bgColor: "bg-status-queued-bg",
    textColor: "text-status-queued",
  },
  draft: {
    label: "Draft",
    dotColor: "bg-status-draft",
    bgColor: "bg-status-draft-bg",
    textColor: "text-status-draft",
  },
  published: {
    label: "Published",
    dotColor: "bg-status-published",
    bgColor: "bg-status-published-bg",
    textColor: "text-status-published",
  },
  archived: {
    label: "Archived",
    dotColor: "bg-status-draft",
    bgColor: "bg-status-draft-bg",
    textColor: "text-status-draft",
  },
  pending: {
    label: "Pending",
    dotColor: "bg-status-queued",
    bgColor: "bg-status-queued-bg",
    textColor: "text-status-queued",
  },
  verified: {
    label: "Verified",
    dotColor: "bg-status-delivered",
    bgColor: "bg-status-delivered-bg",
    textColor: "text-status-delivered",
  },
  error: {
    label: "Error",
    dotColor: "bg-status-bounced",
    bgColor: "bg-status-bounced-bg",
    textColor: "text-status-bounced",
  },
  active: {
    label: "Active",
    dotColor: "bg-status-delivered",
    bgColor: "bg-status-delivered-bg",
    textColor: "text-status-delivered",
  },
  disabled: {
    label: "Disabled",
    dotColor: "bg-status-draft",
    bgColor: "bg-status-draft-bg",
    textColor: "text-status-draft",
  },
  revoked: {
    label: "Revoked",
    dotColor: "bg-status-bounced",
    bgColor: "bg-status-bounced-bg",
    textColor: "text-status-bounced",
  },
};

interface StatusBadgeProps {
  status: Status;
  className?: string;
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const config = statusConfig[status];

  return (
    <span
      className={cn(
        "inline-flex items-center justify-center gap-1.5 rounded-full px-2.5 h-6 font-mono text-[11px] font-medium",
        config.bgColor,
        config.textColor,
        className
      )}
    >
      <span className={cn("h-1.5 w-1.5 rounded-full", config.dotColor)} />
      {config.label}
    </span>
  );
}

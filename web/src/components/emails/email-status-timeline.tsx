"use client";

import { cn } from "@/lib/utils";
import { formatDate } from "@/lib/utils";
import type { EmailEvent } from "@/types/emails";
import type { EmailStatus } from "@/types/api";

const eventDotColor: Record<EmailStatus, string> = {
  queued: "bg-status-queued",
  processing: "bg-status-queued",
  sent: "bg-status-sent",
  delivered: "bg-status-delivered",
  opened: "bg-status-delivered",
  bounced: "bg-status-bounced",
  complained: "bg-status-complained",
  failed: "bg-status-bounced",
  suppressed: "bg-status-queued",
};

const eventLabel: Record<EmailStatus, string> = {
  queued: "Queued",
  processing: "Processing",
  sent: "Sent to provider",
  delivered: "Delivered",
  opened: "Opened",
  bounced: "Bounced",
  complained: "Complained",
  failed: "Failed",
  suppressed: "Suppressed",
};

interface EmailStatusTimelineProps {
  events: EmailEvent[];
}

export function EmailStatusTimeline({ events }: EmailStatusTimelineProps) {
  // Sort events newest first
  const sorted = [...(events ?? [])].sort(
    (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
  );

  return (
    <div className="flex flex-col gap-4">
      {sorted.map((event, idx) => (
        <div key={event.id} className="flex items-start gap-3">
          <div className="flex flex-col items-center">
            <span
              className={cn(
                "h-2.5 w-2.5 rounded-full mt-1.5",
                eventDotColor[event.event_type] ?? "bg-status-draft"
              )}
            />
            {idx < sorted.length - 1 && (
              <div className="w-px flex-1 bg-border min-h-[24px]" />
            )}
          </div>
          <div className="flex flex-col gap-0.5 pb-4">
            <span className="text-[13px] font-medium font-[Sora]">
              {eventLabel[event.event_type] ?? event.event_type}
            </span>
            <span className="text-[11px] font-mono text-muted-foreground">
              {formatDate(event.timestamp)}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}

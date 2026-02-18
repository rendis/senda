"use client";

import { useEffect, useState } from "react";
import { useMinimumLoading } from "@/hooks/use-minimum-loading";
import { useParams } from "next/navigation";
import { ChevronRight, Braces, Puzzle, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { useEmailDetail } from "@/hooks/use-email-detail";
import { StatusBadge } from "@/components/shared/status-badge";
import { EmptyState } from "@/components/shared/empty-state";
import { EmailStatusTimeline } from "@/components/emails/email-status-timeline";
import { EmailDetailPanel } from "@/components/emails/email-detail-panel";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";

function DetailSkeleton() {
  return (
    <div className="flex gap-8">
      <div className="flex-1 flex flex-col gap-6">
        <div className="flex items-center gap-4">
          <Skeleton className="h-6 w-20 rounded-full" />
          <Skeleton className="h-5 w-48" />
        </div>
        <Skeleton className="h-7 w-96" />
        <div className="rounded-lg border bg-card p-6">
          <Skeleton className="h-4 w-40 mb-4" />
          {Array.from({ length: 7 }).map((_, i) => (
            <div key={i} className="flex gap-2 mb-3">
              <Skeleton className="h-4 w-[120px]" />
              <Skeleton className="h-4 w-48" />
            </div>
          ))}
        </div>
      </div>
      <div className="w-[320px] flex flex-col gap-6">
        <div className="rounded-lg border bg-card p-6">
          <Skeleton className="h-4 w-20 mb-4" />
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="flex items-start gap-3 mb-4">
              <Skeleton className="h-2.5 w-2.5 rounded-full mt-1" />
              <div className="flex flex-col gap-1">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-3 w-36" />
              </div>
            </div>
          ))}
        </div>
        <div className="rounded-lg border bg-card p-6">
          <Skeleton className="h-4 w-24 mb-3" />
          <Skeleton className="h-8 w-full mb-2" />
          <Skeleton className="h-8 w-full" />
        </div>
      </div>
    </div>
  );
}

interface CollapsibleJsonProps {
  icon: React.ElementType;
  label: string;
  data: Record<string, unknown> | undefined;
}

function CollapsibleJson({ icon: Icon, label, data }: CollapsibleJsonProps) {
  const [open, setOpen] = useState(false);

  if (!data) return null;

  return (
    <div>
      <button
        className="flex items-center gap-2 w-full text-left hover:bg-muted/50 rounded px-1 py-1 transition-colors"
        onClick={() => setOpen(!open)}
      >
        <Icon className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-[13px] font-[Sora] text-foreground">{label}</span>
        <ChevronRight
          className={`h-3.5 w-3.5 text-muted-foreground ml-auto transition-transform ${
            open ? "rotate-90" : ""
          }`}
        />
      </button>
      {open && (
        <pre className="mt-2 rounded-md bg-muted/50 p-3 text-xs font-mono text-foreground overflow-x-auto max-h-64">
          {JSON.stringify(data, null, 2)}
        </pre>
      )}
    </div>
  );
}

export function EmailDetailContent() {
  const params = useParams<{ trackingId: string }>();
  const { data: email, isLoading: rawLoading, error, refetch } = useEmailDetail(
    params.trackingId
  );
  const isLoading = useMinimumLoading(rawLoading);

  useEffect(() => {
    if (error) toast.error("Failed to load email details");
  }, [error]);

  if (isLoading) return <DetailSkeleton />;

  if (error && !email) {
    return (
      <EmptyState
        icon={RefreshCw}
        title="Failed to load email"
        description="Something went wrong while loading email details. Please try again."
        action={
          <Button variant="outline" onClick={() => refetch()}>
            Retry
          </Button>
        }
      />
    );
  }

  if (!email) return null;

  return (
    <div className="flex gap-8 animate-in fade-in duration-300">
      {/* Left column */}
      <div className="flex-1 flex flex-col gap-6 min-w-0">
        {/* Status + Recipient */}
        <div className="flex items-center gap-4">
          <StatusBadge status={email.status} />
          <span className="font-mono text-base font-semibold text-foreground">
            {email.recipient_email}
          </span>
        </div>

        {/* Subject */}
        <h2
          className="text-xl font-semibold font-[Sora]"
          style={{ letterSpacing: "-1px" }}
        >
          {email.subject}
        </h2>

        {/* Info card */}
        <EmailDetailPanel email={email} />

        {/* Error section for bounced/failed */}
        {(email.status === "bounced" || email.status === "failed") &&
          email.events?.some(
            (e) =>
              (e.event_type === "bounced" || e.event_type === "failed") &&
              e.metadata
          ) && (
            <div className="rounded-lg border border-red-200 bg-red-50 p-6">
              <h3 className="text-sm font-semibold text-red-800 mb-2">
                Error Details
              </h3>
              {(email.events ?? [])
                .filter(
                  (e) =>
                    (e.event_type === "bounced" || e.event_type === "failed") &&
                    e.metadata
                )
                .map((e) => (
                  <pre
                    key={e.id}
                    className="text-xs font-mono text-red-700 whitespace-pre-wrap"
                  >
                    {JSON.stringify(e.metadata, null, 2)}
                  </pre>
                ))}
            </div>
          )}
      </div>

      {/* Right column */}
      <div className="w-[320px] shrink-0 flex flex-col gap-6">
        {/* Timeline */}
        <div className="rounded-lg border bg-card p-6">
          <h3 className="text-sm font-semibold font-[Sora] mb-4">Timeline</h3>
          <EmailStatusTimeline events={email.events ?? []} />
        </div>

        {/* Snapshots */}
        {(email.variables || email.injectors_snapshot) && (
          <div className="rounded-lg border bg-card p-6">
            <h3 className="text-sm font-semibold font-[Sora] mb-3">
              Snapshots
            </h3>
            <div className="flex flex-col gap-2">
              <CollapsibleJson
                icon={Braces}
                label="Variables sent"
                data={email.variables}
              />
              <CollapsibleJson
                icon={Puzzle}
                label="Resolved injectors"
                data={email.injectors_snapshot}
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

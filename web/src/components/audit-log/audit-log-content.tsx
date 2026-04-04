"use client";

import { useState, useEffect, useMemo } from "react";
import { ScrollText } from "lucide-react";
import { toast } from "sonner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { ActionBadge } from "@/components/audit-log/action-badge";
import { ScopeBadge } from "@/components/audit-log/scope-badge";
import { AuditLogDetail } from "@/components/audit-log/audit-log-detail";
import { AuditLogFiltersBar } from "@/components/audit-log/audit-log-filters";
import { useAuditLog } from "@/hooks/use-audit-log";
import type { AuditLogEntry, AuditLogFilters } from "@/types/audit-log";

function formatTimestamp(ts: string): string {
  return new Date(ts).toISOString().slice(0, 16).replace("T", " ");
}

function inferScope(entry: AuditLogEntry): string {
  const et = entry.entity_type.toLowerCase();
  if (et.startsWith("workspace") || et.includes("workspace")) return "workspace";
  if (et.startsWith("tenant") || et.includes("tenant")) return "tenant";
  return "global";
}

function AuditTableSkeleton() {
  return (
    <div className="rounded-lg border bg-card">
      <Table>
        <TableHeader>
          <TableRow>
            {["DATE", "WHO", "ACTION", "RESOURCE", "SCOPE"].map((h) => (
              <TableHead key={h}>
                <Skeleton className="h-4 w-16" />
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {Array.from({ length: 5 }).map((_, i) => (
            <TableRow key={i}>
              {Array.from({ length: 5 }).map((_, j) => (
                <TableCell key={j}>
                  <Skeleton className="h-4 w-full" />
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;

function getDefaultSince(): string {
  return new Date(Date.now() - SEVEN_DAYS_MS).toISOString();
}

export function AuditLogContent() {
  return <AuditLogTable />;
}

function AuditLogTable() {
  const [filters, setFilters] = useState<AuditLogFilters>(() => ({
    since: getDefaultSince(),
  }));
  const [selectedEntry, setSelectedEntry] = useState<AuditLogEntry | null>(
    null
  );
  const [detailOpen, setDetailOpen] = useState(false);

  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useAuditLog(filters);

  useEffect(() => {
    if (error) toast.error("Failed to load audit log");
  }, [error]);

  const allEntries = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data]
  );

  const handleRowClick = (entry: AuditLogEntry) => {
    setSelectedEntry(entry);
    setDetailOpen(true);
  };

  return (
    <div className="flex flex-col gap-6">
      <AuditLogFiltersBar filters={filters} onFiltersChange={setFilters} />

      {isLoading ? (
        <AuditTableSkeleton />
      ) : allEntries.length === 0 ? (
        <EmptyState
          icon={ScrollText}
          title="No events recorded"
          description="No audit events found for the selected filters. Try adjusting the date range or filters."
        />
      ) : (
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[160px]">
                  <span className="font-mono text-[11px] font-semibold tracking-wider uppercase">
                    Date
                  </span>
                </TableHead>
                <TableHead>
                  <span className="font-mono text-[11px] font-semibold tracking-wider uppercase">
                    Who
                  </span>
                </TableHead>
                <TableHead className="w-[120px]">
                  <span className="font-mono text-[11px] font-semibold tracking-wider uppercase">
                    Action
                  </span>
                </TableHead>
                <TableHead className="w-[200px]">
                  <span className="font-mono text-[11px] font-semibold tracking-wider uppercase">
                    Resource
                  </span>
                </TableHead>
                <TableHead className="w-[100px]">
                  <span className="font-mono text-[11px] font-semibold tracking-wider uppercase">
                    Scope
                  </span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {allEntries.map((entry) => (
                <TableRow
                  key={entry.id}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => handleRowClick(entry)}
                >
                  <TableCell>
                    <span className="font-mono text-xs text-muted-foreground">
                      {formatTimestamp(entry.created_at)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-[13px]">
                      {entry.actor_email ?? entry.actor_id}
                    </span>
                  </TableCell>
                  <TableCell>
                    <ActionBadge action={entry.action} />
                  </TableCell>
                  <TableCell>
                    <span className="font-mono text-xs">
                      {entry.entity_type}/{entry.entity_id.slice(0, 8)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <ScopeBadge scope={inferScope(entry)} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {hasNextPage && (
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            className="h-9 gap-2"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
          >
            {isFetchingNextPage ? "Loading..." : "Load more"}
          </Button>
        </div>
      )}

      <AuditLogDetail
        entry={selectedEntry}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />
    </div>
  );
}

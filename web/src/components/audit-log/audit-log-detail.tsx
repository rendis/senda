"use client";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ActionBadge } from "@/components/audit-log/action-badge";
import type { AuditLogEntry } from "@/types/audit-log";

interface AuditLogDetailProps {
  entry: AuditLogEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function AuditLogDetail({
  entry,
  open,
  onOpenChange,
}: AuditLogDetailProps) {
  if (!entry) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-3">
            <span>Audit Event Detail</span>
            <ActionBadge action={entry.action} />
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground font-mono text-xs uppercase tracking-wider">
                Actor
              </span>
              <p className="mt-1">{entry.actor_email ?? entry.actor_id}</p>
            </div>
            <div>
              <span className="text-muted-foreground font-mono text-xs uppercase tracking-wider">
                Timestamp
              </span>
              <p className="mt-1 font-mono text-xs">
                {new Date(entry.timestamp).toLocaleString()}
              </p>
            </div>
            <div>
              <span className="text-muted-foreground font-mono text-xs uppercase tracking-wider">
                Entity Type
              </span>
              <p className="mt-1">{entry.entity_type}</p>
            </div>
            <div>
              <span className="text-muted-foreground font-mono text-xs uppercase tracking-wider">
                Entity ID
              </span>
              <p className="mt-1 font-mono text-xs break-all">
                {entry.entity_id}
              </p>
            </div>
          </div>

          {entry.changes && (
            <div className="space-y-3 pt-2">
              {entry.changes.before && (
                <div>
                  <span className="text-muted-foreground font-mono text-xs uppercase tracking-wider">
                    Before
                  </span>
                  <pre className="mt-1 rounded-lg border bg-muted/50 p-3 text-xs font-mono overflow-x-auto max-h-48">
                    {JSON.stringify(entry.changes.before, null, 2)}
                  </pre>
                </div>
              )}
              {entry.changes.after && (
                <div>
                  <span className="text-muted-foreground font-mono text-xs uppercase tracking-wider">
                    After
                  </span>
                  <pre className="mt-1 rounded-lg border bg-muted/50 p-3 text-xs font-mono overflow-x-auto max-h-48">
                    {JSON.stringify(entry.changes.after, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

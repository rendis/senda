"use client";

import { RefreshCw } from "lucide-react";
import { StatusBadge } from "@/components/shared/status-badge";
import { DnsRecordTable } from "./dns-record-table";
import { Button } from "@/components/ui/button";
import { formatDate } from "@/lib/utils";
import type { Domain } from "@/types/domains";

interface DomainDetailProps {
  domain: Domain;
  onVerify: () => void;
  verifying?: boolean;
}

export function DomainDetail({
  domain,
  onVerify,
  verifying = false,
}: DomainDetailProps) {
  return (
    <div className="flex flex-col gap-6">
      {/* Status header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h3
            className="text-lg font-semibold font-mono"
            style={{ letterSpacing: "-0.5px" }}
          >
            {domain.domain_name}
          </h3>
          <StatusBadge status={domain.status} />
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={onVerify}
          disabled={verifying}
          className="gap-2"
        >
          <RefreshCw className={`h-4 w-4 ${verifying ? "animate-spin" : ""}`} />
          Verify Now
        </Button>
      </div>

      {/* Error message */}
      {domain.last_error && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4">
          <p className="text-sm font-medium text-destructive">
            Verification Error
          </p>
          <p className="text-sm text-destructive/80 mt-1">
            {domain.last_error}
          </p>
        </div>
      )}

      {/* Metadata */}
      <div className="grid grid-cols-2 gap-4 text-sm">
        {domain.verified_at && (
          <div>
            <span className="text-muted-foreground">Verified at:</span>{" "}
            <span className="font-mono text-xs">{formatDate(domain.verified_at)}</span>
          </div>
        )}
        {domain.last_check_at && (
          <div>
            <span className="text-muted-foreground">Last check:</span>{" "}
            <span className="font-mono text-xs">{formatDate(domain.last_check_at)}</span>
          </div>
        )}
        {domain.dkim_selector && (
          <div>
            <span className="text-muted-foreground">DKIM Selector:</span>{" "}
            <span className="font-mono text-xs">{domain.dkim_selector}</span>
          </div>
        )}
      </div>

      {/* DNS Records */}
      <div className="flex flex-col gap-3">
        <h4 className="text-sm font-medium">DNS Records</h4>
        <p className="text-xs text-muted-foreground">
          Add these records to your domain&apos;s DNS configuration to verify ownership and enable DKIM signing.
        </p>
        <DnsRecordTable records={domain.dns_records} />
      </div>
    </div>
  );
}

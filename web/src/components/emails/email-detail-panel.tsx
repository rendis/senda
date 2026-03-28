"use client";

import { Copy } from "lucide-react";
import { toast } from "sonner";
import { copyToClipboard, formatDate } from "@/lib/utils";
import type { EmailDetail } from "@/types/emails";

interface InfoRowProps {
  label: string;
  value: string;
  copiable?: boolean;
  highlight?: boolean;
}

function InfoRow({ label, value, copiable, highlight }: InfoRowProps) {
  const handleCopy = async () => {
    const ok = await copyToClipboard(value);
    if (ok) toast.success("Copied to clipboard");
  };

  return (
    <div className="flex gap-2 items-start">
      <span className="text-[13px] font-medium font-[Sora] text-muted-foreground w-[120px] shrink-0">
        {label}
      </span>
      <div className="flex items-center gap-2 min-w-0">
        <span
          className={`font-mono text-[13px] break-all ${highlight ? "text-scope-workspace" : "text-foreground"}`}
        >
          {value}
        </span>
        {copiable && (
          <button
            onClick={handleCopy}
            className="shrink-0 text-muted-foreground hover:text-foreground transition-colors"
            title="Copy to clipboard"
          >
            <Copy className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
    </div>
  );
}

interface EmailDetailPanelProps {
  email: EmailDetail;
}

export function EmailDetailPanel({ email }: EmailDetailPanelProps) {
  return (
    <div className="rounded-lg border bg-card p-6">
      <h3 className="text-sm font-semibold font-[Sora] mb-4">
        Shipping Information
      </h3>
      <div className="flex flex-col gap-3">
        <InfoRow label="To" value={email.recipient_email} />
        <InfoRow
          label="From"
          value={`${email.from_email} (${email.from_name})`}
        />
        <InfoRow
          label="Template"
          value={email.template_type_slug}
          highlight
        />
        <InfoRow label="Adapter" value={email.adapter_id} />
        <InfoRow
          label="Tracking ID"
          value={email.tracking_id}
          copiable
        />
        {email.external_id && (
          <InfoRow
            label="External ID"
            value={email.external_id}
            copiable
          />
        )}
        <InfoRow label="Locale" value={email.locale} />
        <InfoRow label="Sent at" value={formatDate(email.created_at)} />
      </div>
    </div>
  );
}

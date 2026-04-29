import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import type { AdapterType } from "@/types/adapters";

function SesIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 14 14"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      <path
        d="M1.5 4.5L7 8l5.5-3.5M2 3h10a1 1 0 011 1v6a1 1 0 01-1 1H2a1 1 0 01-1-1V4a1 1 0 011-1z"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M10 1.5l1.5 1.5L10 4.5"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function GmailIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 14 14"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      <path
        d="M2 3h10a1 1 0 011 1v6a1 1 0 01-1 1H2a1 1 0 01-1-1V4a1 1 0 011-1z"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M1.5 4l5.5 4 5.5-4M1.5 10l4-3M12.5 10l-4-3"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function SmtpIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" className={className}>
      <path d="M2 4h10v6H2z" stroke="currentColor" strokeWidth="1.2" strokeLinejoin="round" />
      <path d="M4 2h6M4 12h6M7 2v2M7 10v2" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  );
}

const typeConfig: Record<AdapterType, { label: string; textColor: string; bgColor: string; icon: ReactNode }> = {
  ses: {
    label: "SES",
    textColor: "text-adapter-ses",
    bgColor: "bg-adapter-ses-bg",
    icon: <SesIcon className="h-3.5 w-3.5" />,
  },
  gmail: {
    label: "Gmail",
    textColor: "text-adapter-gmail",
    bgColor: "bg-adapter-gmail-bg",
    icon: <GmailIcon className="h-3.5 w-3.5" />,
  },
  smtp: {
    label: "SMTP",
    textColor: "text-cyan-300",
    bgColor: "bg-cyan-500/10",
    icon: <SmtpIcon className="h-3.5 w-3.5" />,
  },
};

interface AdapterTypeBadgeProps {
  type: AdapterType;
  className?: string;
}

export function AdapterTypeBadge({ type, className }: AdapterTypeBadgeProps) {
  const config = typeConfig[type] ?? { label: type, textColor: "text-muted-foreground", bgColor: "bg-muted", icon: null };

  return (
    <span
      className={cn(
        "inline-flex items-center justify-center gap-1 rounded px-2 h-[22px] font-mono text-[11px] font-semibold",
        config.bgColor,
        config.textColor,
        className
      )}
    >
      {config.icon}
      {config.label}
    </span>
  );
}

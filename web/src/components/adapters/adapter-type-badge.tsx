import { cn } from "@/lib/utils";
import type { AdapterType } from "@/types/adapters";

const typeConfig: Record<AdapterType, { label: string; textColor: string; bgColor: string }> = {
  ses: {
    label: "SES",
    textColor: "text-adapter-ses",
    bgColor: "bg-adapter-ses-bg",
  },
  gmail: {
    label: "Gmail",
    textColor: "text-adapter-gmail",
    bgColor: "bg-adapter-gmail-bg",
  },
};

interface AdapterTypeBadgeProps {
  type: AdapterType;
  className?: string;
}

export function AdapterTypeBadge({ type, className }: AdapterTypeBadgeProps) {
  const config = typeConfig[type] ?? { label: type, textColor: "text-muted-foreground", bgColor: "bg-muted" };

  return (
    <span
      className={cn(
        "inline-flex items-center justify-center rounded px-2 h-[22px] font-mono text-[11px] font-semibold",
        config.bgColor,
        config.textColor,
        className
      )}
    >
      {config.label}
    </span>
  );
}

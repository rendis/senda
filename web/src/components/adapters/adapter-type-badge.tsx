import { cn } from "@/lib/utils";
import type { AdapterType } from "@/types/adapters";

const typeConfig: Record<AdapterType, { label: string; textColor: string; bgColor: string }> = {
  ses: {
    label: "SES",
    textColor: "text-orange-700",
    bgColor: "bg-orange-50",
  },
  gmail: {
    label: "Gmail",
    textColor: "text-red-600",
    bgColor: "bg-red-50",
  },
};

interface AdapterTypeBadgeProps {
  type: AdapterType;
  className?: string;
}

export function AdapterTypeBadge({ type, className }: AdapterTypeBadgeProps) {
  const config = typeConfig[type] ?? { label: type, textColor: "text-gray-700", bgColor: "bg-gray-100" };

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

import { cn } from "@/lib/utils";

const actionColors: Record<string, { bg: string; text: string }> = {
  create: { bg: "bg-status-delivered-bg", text: "text-status-delivered" },
  update: { bg: "bg-status-complained-bg", text: "text-status-complained" },
  delete: { bg: "bg-status-bounced-bg", text: "text-status-bounced" },
};

const defaultColor = { bg: "bg-status-draft-bg", text: "text-status-draft" };

interface ActionBadgeProps {
  action: string;
  className?: string;
}

export function ActionBadge({ action, className }: ActionBadgeProps) {
  const colors = actionColors[action] ?? defaultColor;

  return (
    <span
      className={cn(
        "inline-flex items-center justify-center rounded px-2 h-[22px] font-mono text-[11px] font-semibold",
        colors.bg,
        colors.text,
        className
      )}
    >
      {action}
    </span>
  );
}

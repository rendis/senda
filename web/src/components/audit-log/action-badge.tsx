import { cn } from "@/lib/utils";

const actionColors: Record<string, { bg: string; text: string }> = {
  create: { bg: "bg-blue-100", text: "text-blue-700" },
  update: { bg: "bg-yellow-100", text: "text-yellow-700" },
  delete: { bg: "bg-red-50", text: "text-red-600" },
};

const defaultColor = { bg: "bg-gray-100", text: "text-gray-600" };

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

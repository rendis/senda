"use client";

import { Calendar } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DateRange } from "@/hooks/use-dashboard-stats";

interface DateRangePickerProps {
  value: DateRange;
  onChange: (range: DateRange) => void;
}

export function DateRangePicker({ value, onChange }: DateRangePickerProps) {
  const label = value === "7d" ? "Last 7 days" : "Last 30 days";

  return (
    <Button
      variant="outline"
      size="sm"
      className="h-9 gap-2 rounded-md"
      onClick={() => onChange(value === "7d" ? "30d" : "7d")}
    >
      <Calendar className="h-4 w-4 text-muted-foreground" />
      <span className="text-[13px] font-medium">{label}</span>
    </Button>
  );
}

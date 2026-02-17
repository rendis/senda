"use client";

import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { AuditLogFilters } from "@/types/audit-log";

const actionOptions = [
  { value: "all", label: "All" },
  { value: "create", label: "Create" },
  { value: "update", label: "Update" },
  { value: "delete", label: "Delete" },
];

const dateOptions = [
  { value: "all", label: "All time" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "90d", label: "Last 90 days" },
];

interface AuditLogFiltersBarProps {
  filters: AuditLogFilters;
  onFiltersChange: (filters: AuditLogFilters) => void;
}

function getDateRange(value: string): { since?: string; until?: string } {
  if (value === "all") return {};
  const days = parseInt(value);
  const since = new Date();
  since.setDate(since.getDate() - days);
  return { since: since.toISOString() };
}

export function AuditLogFiltersBar({
  filters,
  onFiltersChange,
}: AuditLogFiltersBarProps) {
  return (
    <div className="flex items-center gap-3 w-full">
      <div className="relative w-60">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
        <Input
          placeholder="Search..."
          className="h-9 pl-9 text-[13px]"
          onChange={(e) => {
            const val = e.target.value.trim();
            onFiltersChange({
              ...filters,
              entity_type: val || undefined,
            });
          }}
        />
      </div>

      <div className="flex flex-col gap-1.5 w-[220px]">
        <label className="text-[13px] font-medium">Action</label>
        <Select
          defaultValue="all"
          onValueChange={(val) =>
            onFiltersChange({
              ...filters,
              action: val === "all" ? undefined : val,
            })
          }
        >
          <SelectTrigger className="h-9 text-[13px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {actionOptions.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-col gap-1.5 w-[220px]">
        <label className="text-[13px] font-medium">Date</label>
        <Select
          defaultValue="7d"
          onValueChange={(val) => {
            const range = getDateRange(val);
            onFiltersChange({
              ...filters,
              since: range.since,
              until: range.until,
            });
          }}
        >
          <SelectTrigger className="h-9 text-[13px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {dateOptions.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}

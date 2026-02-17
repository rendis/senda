"use client";

import { useState, useEffect, useCallback } from "react";
import { Search, Calendar } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { EmailStatus } from "@/types/api";
import type { EmailFilters as EmailFiltersType } from "@/types/emails";

const EMAIL_STATUSES: { value: EmailStatus; label: string }[] = [
  { value: "queued", label: "Queued" },
  { value: "processing", label: "Processing" },
  { value: "sent", label: "Sent" },
  { value: "delivered", label: "Delivered" },
  { value: "opened", label: "Opened" },
  { value: "bounced", label: "Bounced" },
  { value: "complained", label: "Complained" },
  { value: "failed", label: "Failed" },
  { value: "suppressed", label: "Suppressed" },
];

const DATE_RANGES = [
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "90d", label: "Last 90 days" },
  { value: "all", label: "All time" },
] as const;

type DateRangeValue = (typeof DATE_RANGES)[number]["value"];

function getDateRange(range: DateRangeValue): { since?: string; until?: string } {
  if (range === "all") return {};
  const now = new Date();
  const days = range === "7d" ? 7 : range === "30d" ? 30 : 90;
  const since = new Date(now.getTime() - days * 24 * 60 * 60 * 1000);
  return { since: since.toISOString(), until: now.toISOString() };
}

interface EmailFiltersBarProps {
  filters: EmailFiltersType;
  onFiltersChange: (filters: EmailFiltersType) => void;
}

export function EmailFiltersBar({ filters, onFiltersChange }: EmailFiltersBarProps) {
  const [searchInput, setSearchInput] = useState(filters.search ?? "");
  const [dateRange, setDateRange] = useState<DateRangeValue>("7d");

  // Debounced search (300ms)
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchInput !== (filters.search ?? "")) {
        onFiltersChange({ ...filters, search: searchInput || undefined });
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleStatusChange = useCallback(
    (value: string) => {
      if (value === "all") {
        onFiltersChange({ ...filters, status: undefined });
      } else {
        onFiltersChange({ ...filters, status: [value as EmailStatus] });
      }
    },
    [filters, onFiltersChange]
  );

  const handleTemplateChange = useCallback(
    (value: string) => {
      onFiltersChange({
        ...filters,
        template_type: value === "all" ? undefined : value,
      });
    },
    [filters, onFiltersChange]
  );

  const handleDateRangeChange = useCallback(
    (value: string) => {
      const range = value as DateRangeValue;
      setDateRange(range);
      const { since, until } = getDateRange(range);
      onFiltersChange({ ...filters, since, until });
    },
    [filters, onFiltersChange]
  );

  return (
    <div className="flex items-center gap-3">
      {/* Search */}
      <div className="relative w-[280px]">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
        <Input
          placeholder="Search by email, tracking ID..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          className="pl-9 h-9 text-[13px] font-[Sora]"
        />
      </div>

      {/* Status filter */}
      <Select
        value={filters.status?.[0] ?? "all"}
        onValueChange={handleStatusChange}
      >
        <SelectTrigger className="w-[160px] h-9">
          <SelectValue placeholder="Status" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All statuses</SelectItem>
          {EMAIL_STATUSES.map((s) => (
            <SelectItem key={s.value} value={s.value}>
              {s.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Template filter */}
      <Select
        value={filters.template_type ?? "all"}
        onValueChange={handleTemplateChange}
      >
        <SelectTrigger className="w-[160px] h-9">
          <SelectValue placeholder="Template" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All templates</SelectItem>
        </SelectContent>
      </Select>

      {/* Date range */}
      <Select value={dateRange} onValueChange={handleDateRangeChange}>
        <SelectTrigger className="h-9 gap-2">
          <Calendar className="h-4 w-4 text-muted-foreground" />
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {DATE_RANGES.map((r) => (
            <SelectItem key={r.value} value={r.value}>
              {r.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

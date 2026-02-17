"use client";

import { useApi } from "@/hooks/use-api";
import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useScopedPath } from "@/hooks/use-scope";
import type { AuditLogEntry, AuditLogFilters } from "@/types/audit-log";

export function useAuditLog(filters: AuditLogFilters = {}) {
  const api = useApi();
  const scopedPath = useScopedPath();

  const searchParams: Record<string, string> = { limit: "25" };
  if (filters.actor_id) searchParams.actor_id = filters.actor_id;
  if (filters.action) searchParams.action = filters.action;
  if (filters.entity_type) searchParams.entity_type = filters.entity_type;
  if (filters.since) searchParams.since = filters.since;
  if (filters.until) searchParams.until = filters.until;

  return usePaginatedQuery<AuditLogEntry>({
    queryKey: ["audit-log", scopedPath, JSON.stringify(filters)],
    fetcher: (cursor) =>
      api
        .get(`${scopedPath}/audit-log`, {
          searchParams: { ...searchParams, ...(cursor ? { cursor } : {}) },
        })
        .json(),
  });
}

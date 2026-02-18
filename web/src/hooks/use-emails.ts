"use client";

import { usePaginatedQuery } from "@/hooks/use-paginated-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { useScopedPath } from "@/hooks/use-scope";
import type { Email, EmailFilters } from "@/types/emails";
import type { PaginatedResponse } from "@/types/api";

const LIMIT = 25;

export function useEmails(filters: EmailFilters) {
  const api = useApi();
  const ready = useApiReady();
  const scopedPath = useScopedPath();

  const searchParams = new URLSearchParams();
  searchParams.set("limit", String(LIMIT));
  if (filters.status && filters.status.length > 0) {
    searchParams.set("status", filters.status.join(","));
  }
  if (filters.template_type) {
    searchParams.set("template_type", filters.template_type);
  }
  if (filters.since) {
    searchParams.set("since", filters.since);
  }
  if (filters.until) {
    searchParams.set("until", filters.until);
  }
  if (filters.search) {
    searchParams.set("search", filters.search);
  }

  const filterKey = searchParams.toString();

  return usePaginatedQuery<Email>({
    queryKey: ["emails", scopedPath, filterKey],
    enabled: ready,
    fetcher: (cursor?: string) => {
      const params = new URLSearchParams(searchParams);
      if (cursor) params.set("cursor", cursor);
      return api
        .get(`${scopedPath}/emails`, { searchParams: params })
        .json<PaginatedResponse<Email>>();
    },
  });
}

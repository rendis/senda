"use client";

import { useQuery } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import type { DashboardStats } from "@/types/api";

export type DateRange = "7d" | "30d";

export function useDashboardStats(scopedPath: string, range: DateRange = "7d") {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["dashboard-stats", scopedPath, range],
    queryFn: () =>
      api.get(`${scopedPath}/dashboard-stats`, {
        searchParams: { range },
      }).json<DashboardStats>(),
    enabled: ready,
  });
}

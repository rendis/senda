"use client";

import { useQuery } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import type { DashboardStats } from "@/types/api";

export type DateRange = "7d" | "30d";

export function useDashboardStats(scopedPath: string, range: DateRange = "7d") {
  const api = useApi();

  return useQuery({
    queryKey: ["dashboard-stats", scopedPath, range],
    queryFn: () =>
      api.get(`${scopedPath}/dashboard-stats`, {
        searchParams: { range },
      }).json<DashboardStats>(),
  });
}

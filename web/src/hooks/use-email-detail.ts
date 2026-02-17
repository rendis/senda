"use client";

import { useQuery } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import { useScopedPath } from "@/hooks/use-scope";
import type { EmailDetail } from "@/types/emails";

export function useEmailDetail(trackingId: string) {
  const api = useApi();
  const scopedPath = useScopedPath();

  return useQuery({
    queryKey: ["email-detail", scopedPath, trackingId],
    queryFn: () =>
      api
        .get(`${scopedPath}/emails/${trackingId}`)
        .json<EmailDetail>(),
    enabled: !!trackingId,
  });
}

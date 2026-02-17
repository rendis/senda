"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import type { PaginatedResponse } from "@/types/api";

interface UsePaginatedQueryOptions<T> {
  queryKey: string[];
  fetcher: (cursor?: string) => Promise<PaginatedResponse<T>>;
  enabled?: boolean;
}

export function usePaginatedQuery<T>({
  queryKey,
  fetcher,
  enabled = true,
}: UsePaginatedQueryOptions<T>) {
  return useInfiniteQuery({
    queryKey,
    queryFn: ({ pageParam }) => fetcher(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) =>
      lastPage.has_more ? lastPage.next_cursor : undefined,
    enabled,
  });
}

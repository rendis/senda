"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import type { SystemSettings, UpdateSettingsRequest } from "@/types/settings";

export function useSettings() {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["settings"],
    queryFn: () => api.get("manage/config").json<SystemSettings>(),
    enabled: ready,
  });
}

export function useUpdateSettings() {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateSettingsRequest) =>
      api.put("manage/config", { json: data }).json<SystemSettings>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
  });
}

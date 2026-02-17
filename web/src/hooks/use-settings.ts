"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import type { SystemSettings, UpdateSettingsRequest } from "@/types/settings";

export function useSettings() {
  const api = useApi();

  return useQuery({
    queryKey: ["settings"],
    queryFn: () => api.get("manage/settings").json<SystemSettings>(),
  });
}

export function useUpdateSettings() {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateSettingsRequest) =>
      api.put("manage/settings", { json: data }).json<SystemSettings>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
  });
}

"use client";

import { useCallback } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { applyEnvironmentSearchParam, normalizeEnvironment } from "@/lib/environment-mode";
import type { Environment } from "@/types/api";

export function useEnvironmentMode() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const environment = normalizeEnvironment(searchParams.get("environment"));

  const setEnvironment = useCallback(
    (nextEnvironment: Environment) => {
      const currentPath = `${pathname}${searchParams.toString() ? `?${searchParams.toString()}` : ""}`;
      router.replace(applyEnvironmentSearchParam(currentPath, nextEnvironment));
    },
    [pathname, router, searchParams],
  );

  return {
    environment,
    isTest: environment === "test",
    setEnvironment,
  };
}

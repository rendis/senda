import type { Environment } from "../types/api.ts";

export const DEFAULT_ENVIRONMENT: Environment = "prod";

export function normalizeEnvironment(
  value?: string | null,
): Environment {
  return value === "test" ? "test" : DEFAULT_ENVIRONMENT;
}

export function applyEnvironmentSearchParam(
  path: string,
  environment?: string | null,
): string {
  const normalized = normalizeEnvironment(environment);
  const url = new URL(path, "https://local");
  url.searchParams.set("environment", normalized);
  return `${url.pathname}${url.search}${url.hash}`;
}

export function resolveEnvironmentSwitchPath(
  path: string,
  environment?: string | null,
): string {
  const normalized = normalizeEnvironment(environment);
  const url = new URL(path, "https://local");
  const match = url.pathname.match(/^\/t\/([^/]+)\/w\/([^/]+)/);

  if (!match) {
    return applyEnvironmentSearchParam(path, normalized);
  }

  return applyEnvironmentSearchParam(`/t/${match[1]}/w/${match[2]}`, normalized);
}

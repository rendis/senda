import { normalizeEnvironment } from "./environment-mode.ts";
import type { Environment } from "../types/api.ts";

export interface ScopedPathParams {
  profileSlug?: string;
  tenantCode?: string;
  workspaceCode?: string;
  environment?: Environment;
}

export interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export const EXTERNAL_EMBED_TOKEN_STORAGE_KEY = "senda.external.embed.token";
export const EXTERNAL_EMBED_TOKEN_CHANGED_EVENT =
  "senda:external-embed-token-changed";
export const EXTERNAL_EMBED_TOKEN_HEADER = "x-senda-external-token";
export const EXTERNAL_TENANT_HEADER = "x-tenant-code";
export const EXTERNAL_ENVIRONMENT_HEADER = "x-senda-environment";

function normalizeSearchString(currentSearch: string): string {
  if (!currentSearch) {
    return "";
  }

  return currentSearch.startsWith("?") ? currentSearch : `?${currentSearch}`;
}

function externalEmbedURL(currentSearch: string): URL {
  return new URL(`https://local${normalizeSearchString(currentSearch)}`);
}

function normalizeToken(value: string | null | undefined): string | null {
  const token = value?.trim() ?? "";
  return token || null;
}

function getDefaultExternalEmbedTokenStorage(): StorageLike | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }

  return window.sessionStorage;
}

function isExternalAPIRequest(requestURL: string): boolean {
  return new URL(requestURL, "https://local").pathname.startsWith(
    "/api/v1/external/",
  );
}

const FORWARDED_EXTERNAL_QUERY_PARAMS = new Set(["fallback", "readonly"]);

interface ExternalEmbedPathContext {
  profileSlug: string;
  environment: Environment;
  tenantCode: string | null;
}

function parseExternalEmbedPath(
  currentPathname: string,
): ExternalEmbedPathContext | null {
  const segments = currentPathname.split("/").filter(Boolean);
  if (segments.length < 4 || segments[0] !== "embed" || !segments[1]) {
    return null;
  }

  let index = 2;
  let environment: Environment = "prod";
  if (segments[index] === "environments") {
    environment = normalizeEnvironment(segments[index + 1]);
    index += 2;
  }

  if (segments[index] !== "t") {
    return {
      profileSlug: segments[1],
      environment,
      tenantCode: null,
    };
  }

  const tenantCode = segments[index + 1]?.trim() || null;
  return {
    profileSlug: segments[1],
    environment,
    tenantCode,
  };
}

function extractTenantCodeFromEmbedPath(currentPathname: string): string | null {
  return parseExternalEmbedPath(currentPathname)?.tenantCode ?? null;
}

export function extractEnvironmentFromEmbedPath(
  currentPathname: string,
): Environment {
  return parseExternalEmbedPath(currentPathname)?.environment ?? "prod";
}

export function resolveScopedPathFromParams(params: ScopedPathParams): string {
  if (params.profileSlug && params.tenantCode && params.workspaceCode) {
    return `external/${params.profileSlug}/tenants/${params.tenantCode}/workspaces/${params.workspaceCode}`;
  }

  if (params.tenantCode && params.workspaceCode) {
    return `manage/environments/${normalizeEnvironment(params.environment)}/tenants/${params.tenantCode}/workspaces/${params.workspaceCode}`;
  }

  if (params.tenantCode) {
    return `manage/tenants/${params.tenantCode}`;
  }

  return "manage/global";
}

export function resolveWorkspacePoliciesPathFromParams(
  params: ScopedPathParams,
): string | null {
  if (!params.tenantCode || !params.workspaceCode) {
    return null;
  }

  if (params.profileSlug) {
    return `external/${params.profileSlug}/tenants/${params.tenantCode}/workspaces/${params.workspaceCode}/policies`;
  }

  return `manage/environments/${normalizeEnvironment(params.environment)}/tenants/${params.tenantCode}/workspaces/${params.workspaceCode}/policies`;
}

export function isExternalEmbedPath(currentPathname: string): boolean {
  return currentPathname.startsWith("/embed/");
}

export function extractExternalEmbedTokenFromSearch(
  currentSearch: string,
): string | null {
  const current = externalEmbedURL(currentSearch);
  return normalizeToken(current.searchParams.get("token"));
}

export function stripExternalEmbedTokenFromSearch(
  currentSearch: string,
): string {
  const url = externalEmbedURL(currentSearch);
  url.searchParams.delete("token");
  const search = url.searchParams.toString();
  return search ? `?${search}` : "";
}

export function readExternalEmbedToken(
  storage: StorageLike | undefined = getDefaultExternalEmbedTokenStorage(),
): string | null {
  if (!storage) {
    return null;
  }

  return normalizeToken(storage.getItem(EXTERNAL_EMBED_TOKEN_STORAGE_KEY));
}

export function storeExternalEmbedToken(
  token: string,
  storage: StorageLike | undefined = getDefaultExternalEmbedTokenStorage(),
): boolean {
  const normalized = normalizeToken(token);
  if (!normalized || !storage) {
    return false;
  }

  storage.setItem(EXTERNAL_EMBED_TOKEN_STORAGE_KEY, normalized);
  return true;
}

export function clearExternalEmbedToken(
  storage: StorageLike | undefined = getDefaultExternalEmbedTokenStorage(),
): boolean {
  if (!storage) {
    return false;
  }

  storage.removeItem(EXTERNAL_EMBED_TOKEN_STORAGE_KEY);
  return true;
}

export function captureExternalEmbedTokenFromSearch(
  currentSearch: string,
  storage: StorageLike | undefined = getDefaultExternalEmbedTokenStorage(),
): { token: string | null; cleanedSearch: string } {
  const token = extractExternalEmbedTokenFromSearch(currentSearch);
  if (token) {
    storeExternalEmbedToken(token, storage);
  }

  return {
    token,
    cleanedSearch: stripExternalEmbedTokenFromSearch(currentSearch),
  };
}

export function isExternalEmbedRuntimeReady(
  currentPathname: string,
  storage: StorageLike | undefined = getDefaultExternalEmbedTokenStorage(),
): boolean {
  if (!isExternalEmbedPath(currentPathname)) {
    return false;
  }

  return readExternalEmbedToken(storage) !== null;
}

export function buildExternalEmbedApiRequest(
  request: Request,
  currentPathname: string,
  storage: StorageLike | undefined = getDefaultExternalEmbedTokenStorage(),
  currentSearch = "",
): Request | undefined {
  if (!isExternalEmbedPath(currentPathname)) {
    return undefined;
  }

  if (!isExternalAPIRequest(request.url)) {
    return undefined;
  }

  const token = readExternalEmbedToken(storage);
  if (!token) {
    return undefined;
  }

  const headers = new Headers(request.headers);
  headers.set(EXTERNAL_EMBED_TOKEN_HEADER, token);
  const tenantCode = extractTenantCodeFromEmbedPath(currentPathname);
  if (tenantCode) {
    headers.set(EXTERNAL_TENANT_HEADER, tenantCode);
  }
  headers.set(
    EXTERNAL_ENVIRONMENT_HEADER,
    extractEnvironmentFromEmbedPath(currentPathname),
  );

  const url = new URL(request.url);
  const current = externalEmbedURL(currentSearch);
  for (const [key, value] of current.searchParams.entries()) {
    if (!FORWARDED_EXTERNAL_QUERY_PARAMS.has(key)) {
      continue;
    }
    if (!url.searchParams.has(key)) {
      url.searchParams.set(key, value);
    }
  }

  if (url.toString() === request.url) {
    headers.forEach((value, key) => {
      request.headers.set(key, value);
    });
    return request;
  }

  const nextRequest = new Request(url, request);
  headers.forEach((value, key) => {
    nextRequest.headers.set(key, value);
  });
  return nextRequest;
}

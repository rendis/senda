"use client";

const basePath = process.env.__NEXT_ROUTER_BASEPATH || "";

let logoutStarted = false;

function sanitizeCallbackUrl(callbackUrl?: string): string {
  if (typeof window === "undefined") {
    return `${basePath}/login`;
  }

  try {
    const raw = callbackUrl ?? `${basePath}/login`;
    const url = new URL(raw, window.location.origin);
    if (url.origin !== window.location.origin) {
      return `${basePath}/login`;
    }
    return `${url.pathname}${url.search}${url.hash}` || `${basePath}/login`;
  } catch {
    return `${basePath}/login`;
  }
}

export function startFederatedLogout(callbackUrl?: string) {
  if (typeof window === "undefined" || logoutStarted) {
    return;
  }

  logoutStarted = true;
  const safeCallbackUrl = sanitizeCallbackUrl(callbackUrl ?? `${basePath}/login`);
  window.location.assign(
    `${basePath}/logout?callbackUrl=${encodeURIComponent(safeCallbackUrl)}`,
  );
}

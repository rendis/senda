"use client";

let logoutStarted = false;

function sanitizeCallbackUrl(callbackUrl?: string): string {
  if (typeof window === "undefined") {
    return "/login";
  }

  try {
    const url = new URL(callbackUrl ?? "/login", window.location.origin);
    if (url.origin !== window.location.origin) {
      return "/login";
    }
    return `${url.pathname}${url.search}${url.hash}` || "/login";
  } catch {
    return "/login";
  }
}

export function startFederatedLogout(callbackUrl = "/login") {
  if (typeof window === "undefined" || logoutStarted) {
    return;
  }

  logoutStarted = true;
  const safeCallbackUrl = sanitizeCallbackUrl(callbackUrl);
  window.location.assign(
    `/logout?callbackUrl=${encodeURIComponent(safeCallbackUrl)}`,
  );
}

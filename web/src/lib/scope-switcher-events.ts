import { useCallback, useEffect } from "react";

type OpenPayload = {
  view: "tenants" | "workspaces";
  tenantCode?: string;
  tenantName?: string;
};

const EVENT_NAME = "scope-switcher:open";

export function emitOpenScopeSwitcher(payload: OpenPayload) {
  window.dispatchEvent(new CustomEvent(EVENT_NAME, { detail: payload }));
}

export function useOnScopeSwitcherOpen(
  callback: (payload: OpenPayload) => void
) {
  const stableCallback = useCallback((p: OpenPayload) => callback(p), [callback]);

  useEffect(() => {
    const handler = (e: Event) =>
      stableCallback((e as CustomEvent<OpenPayload>).detail);
    window.addEventListener(EVENT_NAME, handler);
    return () => window.removeEventListener(EVENT_NAME, handler);
  }, [stableCallback]);
}

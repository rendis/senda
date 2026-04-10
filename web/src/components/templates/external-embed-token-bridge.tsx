"use client";

import { useEffect } from "react";
import {
  captureExternalEmbedTokenFromSearch,
  EXTERNAL_EMBED_TOKEN_CHANGED_EVENT,
  isExternalEmbedPath,
} from "@/lib/external-api-context";

export function ExternalEmbedTokenBridge() {
  useEffect(() => {
    if (!isExternalEmbedPath(window.location.pathname)) {
      return;
    }

    const { token, cleanedSearch } = captureExternalEmbedTokenFromSearch(
      window.location.search,
      window.sessionStorage,
    );

    if (token) {
      const nextUrl = new URL(window.location.href);
      nextUrl.search = cleanedSearch;
      window.history.replaceState(window.history.state, "", `${nextUrl.pathname}${nextUrl.search}${nextUrl.hash}`);
      window.dispatchEvent(new Event(EXTERNAL_EMBED_TOKEN_CHANGED_EVENT));
    }
  }, []);

  return null;
}

"use client";

import { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { signOut } from "next-auth/react";
import { Loader2 } from "lucide-react";
import { BrandLogo } from "@/components/shared/brand-logo";

function sanitizeCallbackUrl(callbackUrl: string | null): string {
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

export default function LogoutPage() {
  const searchParams = useSearchParams();

  useEffect(() => {
    let cancelled = false;

    async function performLogout() {
      const callbackUrl = sanitizeCallbackUrl(searchParams.get("callbackUrl"));
      let providerLogoutUrl = callbackUrl;

      try {
        const response = await fetch(
          `/api/auth/federated-logout-url?callbackUrl=${encodeURIComponent(callbackUrl)}`,
          {
            cache: "no-store",
          },
        );

        if (response.ok) {
          const payload = (await response.json()) as { url?: string };
          if (payload.url) {
            providerLogoutUrl = payload.url;
          }
        }
      } catch {
        providerLogoutUrl = callbackUrl;
      }

      try {
        await signOut({ redirect: false, callbackUrl });
      } catch {
        // Best effort — still continue with provider logout.
      }

      if (!cancelled) {
        window.location.replace(providerLogoutUrl);
      }
    }

    void performLogout();

    return () => {
      cancelled = true;
    };
  }, [searchParams]);

  return (
    <main className="flex min-h-screen items-center justify-center bg-page px-4 py-10 sm:px-6">
      <div className="flex w-full max-w-sm flex-col items-center gap-4 rounded-lg border border-border bg-card px-6 py-8 text-center sm:px-8">
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl border border-border bg-background shadow-sm">
          <BrandLogo size="sm" priority />
        </div>
        <div className="space-y-2">
          <h1 className="text-lg font-semibold text-foreground">
            Signing you out
          </h1>
          <p className="text-sm leading-relaxed text-muted-foreground">
            Closing your Senda session and your identity provider session.
          </p>
        </div>
        <Loader2 className="h-5 w-5 animate-spin text-primary" />
      </div>
    </main>
  );
}

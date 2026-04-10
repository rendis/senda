"use client";

import { SessionProvider } from "next-auth/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "next-themes";
import { TooltipProvider } from "@/components/ui/tooltip";
import { SessionGuard } from "@/providers/session-guard";
import { useState } from "react";
import { usePathname } from "next/navigation";
import { isExternalEmbedPath } from "@/providers/session-guard-policy";

export function Providers({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const externalEmbedMode = isExternalEmbedPath(pathname);
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30 * 1000,
            retry: 1,
          },
        },
      })
  );

  return (
    <SessionProvider
      session={externalEmbedMode ? null : undefined}
      refetchInterval={externalEmbedMode ? 0 : 3 * 60}
      refetchOnWindowFocus={!externalEmbedMode}
    >
      <SessionGuard>
        <QueryClientProvider client={queryClient}>
          <ThemeProvider
            attribute="class"
            defaultTheme="system"
            enableSystem
            disableTransitionOnChange
          >
            <TooltipProvider>{children}</TooltipProvider>
          </ThemeProvider>
        </QueryClientProvider>
      </SessionGuard>
    </SessionProvider>
  );
}

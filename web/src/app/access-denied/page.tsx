"use client";

import { useSession } from "next-auth/react";
import { ShieldX, LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PublicThemeToggle } from "@/components/shared/public-theme-toggle";
import { useTranslations } from "next-intl";
import { startFederatedLogout } from "@/lib/logout";

export default function AccessDeniedPage() {
  const t = useTranslations("accessDenied");
  const { data: session } = useSession();

  return (
    <>
      <PublicThemeToggle />
      <header className="sr-only">
        <nav aria-label="Public navigation">
          <a href={`${process.env.__NEXT_ROUTER_BASEPATH || ""}/login`}>Back to login</a>
        </nav>
      </header>
      <main className="flex min-h-screen items-center justify-center bg-page px-4 py-10 sm:px-6">
        <div className="w-full max-w-[440px] rounded-lg border border-border bg-card px-6 py-10 sm:px-10 sm:py-12">
          <div className="flex flex-col items-center gap-6">
            {/* Icon */}
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-destructive">
              <ShieldX className="h-8 w-8 text-destructive-foreground" />
            </div>

            {/* Title */}
            <h1 className="text-2xl font-bold text-foreground">
              {t("title")}
            </h1>

            {/* Message */}
            <p className="w-full max-w-[360px] text-center text-sm leading-relaxed text-foreground/70">
              {t("message", { email: session?.user?.email ?? "unknown" })}
            </p>

            {/* Divider */}
            <div className="h-px w-full bg-border" />

            {/* Sign Out Button */}
            <Button
              variant="destructive"
              onClick={() => startFederatedLogout("/login")}
              className="h-9 w-full gap-2 rounded-md text-[13px] font-medium"
            >
              <LogOut className="h-4 w-4" />
              {t("signOut")}
            </Button>
          </div>
        </div>
      </main>
    </>
  );
}

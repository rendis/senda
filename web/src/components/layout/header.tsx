"use client";

import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell, PanelLeft } from "lucide-react";
import { useEnvironmentMode } from "@/hooks/use-environment-mode";
import { useScope } from "@/hooks/use-scope";
import { cn } from "@/lib/utils";

export function AppHeader({
  onOpenMobileSidebar,
}: {
  onOpenMobileSidebar?: () => void;
}) {
  const pathname = usePathname();
  const t = useTranslations("nav");
  const scope = useScope();
  const { environment, setEnvironment } = useEnvironmentMode();

  const pathToTitle: Record<string, string> = {
    "": t("dashboard"),
    "/emails": t("emails"),
    "/templates": t("templates"),
    "/injectors": t("injectors"),
    "/adapters": t("adapters"),
    "/webhooks": t("webhooks"),
    "/members": t("members"),
    "/api-keys": t("apiKeys"),
    "/tenants": t("tenants"),
    "/workspaces": t("workspaces"),
    "/api-docs": t("apiDocs"),
    "/audit-log": t("auditLog"),
    "/settings": t("settings"),
    "/help": t("help"),
  };

  // Strip scope prefix: /global, /t/[code], /t/[code]/w/[code]
  const stripped = pathname
    .replace(/^\/global/, "")
    .replace(/^\/t\/[^/]+(\/w\/[^/]+)?/, "");

  // Exact match first
  let title = pathToTitle[stripped];

  if (!title) {
    // Match by prefix (e.g. /emails/123 → "Emails")
    for (const [key, value] of Object.entries(pathToTitle)) {
      if (key && stripped.startsWith(key)) {
        title = value;
        break;
      }
    }
  }

  if (!title) {
    title = t("dashboard");
  }

  return (
    <header className="flex items-center justify-between h-14 px-8 border-b bg-card">
      <div className="flex items-center gap-3">
        <button
          type="button"
          aria-label="Open navigation"
          title="Open navigation"
          onClick={onOpenMobileSidebar}
          className="rounded-md p-2 text-muted-foreground transition-colors hover:text-foreground md:hidden"
        >
          <PanelLeft className="h-5 w-5" />
        </button>
        <h2 className="text-lg font-semibold" style={{ letterSpacing: "-1px" }}>
          {title}
        </h2>
      </div>
      <div className="flex items-center gap-2">
        {scope.level === "workspace" ? (
          <div className="flex items-center gap-2 rounded-full border bg-background px-1 py-1">
            <span
              className={cn(
                "rounded-full px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.08em]",
                environment === "prod"
                  ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200"
                  : "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
              )}
            >
              {environment}
            </span>
            <div className="flex items-center gap-1">
              {(["prod", "test"] as const).map((option) => (
                <button
                  key={option}
                  type="button"
                  aria-pressed={environment === option}
                  onClick={() => setEnvironment(option)}
                  className={cn(
                    "rounded-full px-3 py-1 text-xs font-medium transition-colors",
                    environment === option
                      ? option === "prod"
                        ? "bg-emerald-600 text-white"
                        : "bg-amber-500 text-amber-950"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  {option.toUpperCase()}
                </button>
              ))}
            </div>
          </div>
        ) : null}
        <button
          type="button"
          aria-label="Notifications"
          title="Notifications"
          className="rounded-md p-2 text-muted-foreground hover:text-foreground transition-colors outline-none ring-ring focus-visible:ring-2"
        >
          <Bell className="h-5 w-5" />
        </button>
      </div>
    </header>
  );
}

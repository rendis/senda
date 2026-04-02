"use client";

import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell, PanelLeft } from "lucide-react";

export function AppHeader({
  onOpenMobileSidebar,
}: {
  onOpenMobileSidebar?: () => void;
}) {
  const pathname = usePathname();
  const t = useTranslations("nav");

  const pathToTitle: Record<string, string> = {
    "": t("dashboard"),
    "/emails": t("emails"),
    "/templates": t("templates"),
    "/injectors": t("injectors"),
    "/adapters": t("adapters"),
    "/webhooks": t("webhooks"),
    "/members": t("members"),
    "/api-keys": t("apiKeys"),
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

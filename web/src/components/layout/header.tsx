"use client";

import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell } from "lucide-react";

export function AppHeader() {
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
    "/audit-log": t("auditLog"),
    "/settings": t("settings"),
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
      <h2 className="text-lg font-semibold" style={{ letterSpacing: "-1px" }}>
        {title}
      </h2>
      <div className="flex items-center gap-2">
        <button className="rounded-md p-2 text-muted-foreground hover:text-foreground transition-colors outline-none ring-ring focus-visible:ring-2">
          <Bell className="h-5 w-5" />
        </button>
      </div>
    </header>
  );
}

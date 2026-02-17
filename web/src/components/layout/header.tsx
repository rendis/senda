"use client";

import { usePathname } from "next/navigation";
import { Bell } from "lucide-react";

const pathToTitle: Record<string, string> = {
  "": "Dashboard",
  "/emails": "Emails",
  "/templates": "Templates",
  "/injectors": "Injectors",
  "/adapters": "Adapters",
  "/domains": "Domains",
  "/webhooks": "Webhooks",
  "/members": "Members",
  "/api-keys": "API Keys",
  "/audit-log": "Audit Log",
  "/settings": "Settings",
};

function getPageTitle(pathname: string): string {
  // Strip scope prefix: /global, /t/[code], /t/[code]/w/[code]
  const stripped = pathname
    .replace(/^\/global/, "")
    .replace(/^\/t\/[^/]+(\/w\/[^/]+)?/, "");

  // Exact match first
  if (stripped in pathToTitle) return pathToTitle[stripped];

  // Match by prefix (e.g. /emails/123 → "Emails")
  for (const [key, title] of Object.entries(pathToTitle)) {
    if (key && stripped.startsWith(key)) return title;
  }

  return "Dashboard";
}

export function AppHeader() {
  const pathname = usePathname();
  const title = getPageTitle(pathname);

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

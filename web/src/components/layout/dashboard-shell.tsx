"use client";

import { useState } from "react";
import { AppHeader } from "@/components/layout/header";
import { AppSidebar } from "@/components/layout/sidebar";

export function DashboardShell({
  children,
}: {
  children: React.ReactNode;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <div className="flex h-svh overflow-hidden bg-page">
      <AppSidebar
        collapsed={collapsed}
        onCollapsedChange={setCollapsed}
        mobileOpen={mobileOpen}
        onMobileOpenChange={setMobileOpen}
      />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <AppHeader onOpenMobileSidebar={() => setMobileOpen(true)} />
        <main className="min-h-0 min-w-0 flex-1 overflow-y-auto">
          {children}
        </main>
      </div>
    </div>
  );
}

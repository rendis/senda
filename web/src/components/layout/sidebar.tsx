"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { signOut, useSession } from "next-auth/react";
import {
  LayoutDashboard,
  Mail,
  FileText,
  Database,
  Plug,
  Globe,
  Webhook,
  Users,
  Key,
  ScrollText,
  Settings,
  CircleHelp,
  PanelLeftClose,
  PanelLeft,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useScope } from "@/hooks/use-scope";
import { ScopeSwitcher } from "@/components/shared/scope-switcher";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useState } from "react";

const navItems = [
  { label: "Dashboard", icon: LayoutDashboard, href: "" },
  { label: "Emails", icon: Mail, href: "/emails" },
  { label: "Templates", icon: FileText, href: "/templates" },
  { label: "Injectors", icon: Database, href: "/injectors" },
  { label: "Adapters", icon: Plug, href: "/adapters" },
  { label: "Domains", icon: Globe, href: "/domains" },
  { label: "Webhooks", icon: Webhook, href: "/webhooks" },
  { label: "Members", icon: Users, href: "/members" },
  { label: "API Keys", icon: Key, href: "/api-keys" },
  { label: "Audit Log", icon: ScrollText, href: "/audit-log" },
  { label: "Settings", icon: Settings, href: "/settings" },
] as const;

function getUserInitials(name?: string | null, email?: string | null): string {
  if (name) {
    return name
      .split(" ")
      .filter(Boolean)
      .map((w) => w[0])
      .join("")
      .toUpperCase()
      .slice(0, 2);
  }
  if (email) {
    return email.slice(0, 2).toUpperCase();
  }
  return "??";
}

export function AppSidebar() {
  const pathname = usePathname();
  const { level, tenantCode, workspaceCode } = useScope();
  const { data: session } = useSession();
  const [collapsed, setCollapsed] = useState(false);

  // Build base path for current scope
  let basePath = "/global";
  if (level === "tenant" && tenantCode) {
    basePath = `/t/${tenantCode}`;
  } else if (level === "workspace" && tenantCode && workspaceCode) {
    basePath = `/t/${tenantCode}/w/${workspaceCode}`;
  }

  const displayName = session?.user?.name ?? session?.user?.email ?? "User";
  const initials = getUserInitials(session?.user?.name, session?.user?.email);

  return (
    <aside
      className={cn(
        "flex flex-col justify-between bg-sidebar text-sidebar-foreground transition-all duration-200",
        collapsed ? "w-16" : "w-60"
      )}
      style={{ minHeight: "100vh" }}
    >
      {/* Top */}
      <div className="flex flex-col gap-1">
        {/* Logo */}
        <div className="flex items-center gap-2.5 px-3 h-10 mt-5">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary shrink-0">
            <span className="text-sm font-bold text-primary-foreground">S</span>
          </div>
          {!collapsed && (
            <span className="text-base font-semibold text-white">Senda</span>
          )}
        </div>

        {/* Divider */}
        <div className="mx-3 my-1 h-px bg-sidebar-accent" />

        {/* Scope switcher */}
        <ScopeSwitcher collapsed={collapsed} />

        {/* Divider */}
        <div className="mx-3 my-1 h-px bg-sidebar-accent" />

        {/* Nav items */}
        <nav className="flex flex-col gap-0.5 px-3">
          {navItems.map((item) => {
            const href = `${basePath}${item.href}`;
            const isActive =
              item.href === ""
                ? pathname === basePath || pathname === `${basePath}/`
                : pathname.startsWith(href);

            return (
              <Link
                key={item.label}
                href={href}
                className={cn(
                  "flex items-center gap-2.5 px-3 h-9 rounded-md text-[13px] transition-colors",
                  isActive
                    ? "bg-sidebar-accent text-white font-medium"
                    : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-white"
                )}
              >
                <item.icon className="h-[18px] w-[18px] shrink-0" />
                {!collapsed && <span>{item.label}</span>}
              </Link>
            );
          })}
        </nav>
      </div>

      {/* Bottom */}
      <div className="flex flex-col gap-2 px-3 pb-5">
        {/* Help */}
        <button className="flex items-center gap-2.5 px-3 h-9 rounded-md text-[13px] text-sidebar-foreground hover:bg-sidebar-accent hover:text-white transition-colors">
          <CircleHelp className="h-[18px] w-[18px] shrink-0" />
          {!collapsed && <span>Help</span>}
        </button>

        {/* Divider */}
        <div className="h-px bg-sidebar-accent" />

        {/* User row (dropdown) + collapse */}
        <div
          className={cn(
            "flex px-2",
            collapsed
              ? "flex-col items-center gap-2 py-1"
              : "items-center gap-2.5 h-10"
          )}
        >
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                className={cn(
                  "flex items-center outline-none rounded-md",
                  collapsed ? "justify-center" : "gap-2.5 flex-1 min-w-0"
                )}
              >
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[#334155] shrink-0">
                  <span className="text-[11px] font-semibold text-white">
                    {initials}
                  </span>
                </div>
                {!collapsed && (
                  <span className="text-[13px] text-white truncate">
                    {displayName}
                  </span>
                )}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="start" className="w-48">
              <DropdownMenuItem
                onClick={() => signOut({ callbackUrl: "/login" })}
              >
                <LogOut className="mr-2 h-4 w-4" />
                Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="text-sidebar-foreground hover:text-white transition-colors shrink-0"
          >
            {collapsed ? (
              <PanelLeft className="h-4 w-4" />
            ) : (
              <PanelLeftClose className="h-4 w-4" />
            )}
          </button>
        </div>
      </div>
    </aside>
  );
}

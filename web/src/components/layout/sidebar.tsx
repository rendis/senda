"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import {
  LayoutDashboard,
  Mail,
  FileText,
  Database,
  Plug,
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
import { BrandLogo } from "@/components/shared/brand-logo";
import { ScopeSwitcher } from "@/components/shared/scope-switcher";
import { LocaleSwitcher } from "@/components/shared/locale-switcher";
import { ThemeSelector } from "@/components/shared/theme-selector";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { startFederatedLogout } from "@/lib/logout";

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

interface AppSidebarProps {
  collapsed: boolean;
  onCollapsedChange: (collapsed: boolean) => void;
  mobileOpen: boolean;
  onMobileOpenChange: (open: boolean) => void;
}

export function AppSidebar({
  collapsed,
  onCollapsedChange,
  mobileOpen,
  onMobileOpenChange,
}: AppSidebarProps) {
  const pathname = usePathname();
  const { level, tenantCode, workspaceCode } = useScope();
  const { data: session } = useSession();
  const t = useTranslations("nav");
  const tCommon = useTranslations("common");

  const navItems = [
    { label: t("dashboard"), icon: LayoutDashboard, href: "" },
    { label: t("emails"), icon: Mail, href: "/emails" },
    { label: t("templates"), icon: FileText, href: "/templates" },
    { label: t("injectors"), icon: Database, href: "/injectors" },
    { label: t("adapters"), icon: Plug, href: "/adapters" },
    { label: t("webhooks"), icon: Webhook, href: "/webhooks" },
    { label: t("members"), icon: Users, href: "/members" },
    { label: t("apiKeys"), icon: Key, href: "/api-keys" },
    { label: t("auditLog"), icon: ScrollText, href: "/audit-log" },
    { label: t("settings"), icon: Settings, href: "/settings" },
  ];

  // Build base path for current scope
  let basePath = "/global";
  if (level === "tenant" && tenantCode) {
    basePath = `/t/${tenantCode}`;
  } else if (level === "workspace" && tenantCode && workspaceCode) {
    basePath = `/t/${tenantCode}/w/${workspaceCode}`;
  }

  const displayName = session?.user?.name ?? session?.user?.email ?? "User";
  const initials = getUserInitials(session?.user?.name, session?.user?.email);

  const visibleNavItems = navItems.filter(
    (item) => !(item.href === "/settings" && level !== "global"),
  );

  function hrefForItem(href: string) {
    if (href === "/settings") {
      return "/global/settings";
    }
    return `${basePath}${href}`;
  }

  function renderSidebarContent(
    currentCollapsed: boolean,
    isMobile: boolean,
  ) {
    return (
      <div className="flex h-full flex-col justify-between bg-sidebar text-sidebar-foreground">
        <div className="flex flex-col gap-1">
          <div className="mt-5 flex h-10 items-center gap-2.5 px-3">
            <BrandLogo
              size="sm"
              showWordmark={!currentCollapsed}
              imageClassName="rounded-sm"
              wordmarkClassName="text-base text-sidebar-accent-foreground"
            />
          </div>

          <div className="mx-3 my-1 h-px bg-sidebar-accent" />
          <ScopeSwitcher collapsed={currentCollapsed} />
          <div className="mx-3 my-1 h-px bg-sidebar-accent" />

          <nav className="flex flex-col gap-0.5 px-3">
            {visibleNavItems.map((item) => {
              const href = hrefForItem(item.href);
              const isActive =
                item.href === ""
                  ? pathname === basePath || pathname === `${basePath}/`
                  : pathname.startsWith(href);

              return (
                <Link
                  key={item.href}
                  href={href}
                  onClick={() => {
                    if (isMobile) {
                      onMobileOpenChange(false);
                    }
                  }}
                  className={cn(
                    "flex h-9 items-center gap-2.5 rounded-md px-3 text-[13px] transition-colors",
                    isActive
                      ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                      : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                  )}
                >
                  <item.icon className="h-[18px] w-[18px] shrink-0" />
                  {!currentCollapsed && <span>{item.label}</span>}
                </Link>
              );
            })}
          </nav>
        </div>

        <div className="flex flex-col gap-2 px-3 pb-5">
          <button
            type="button"
            aria-label={t("help")}
            title={t("help")}
            className="flex h-9 items-center gap-2.5 rounded-md px-3 text-[13px] text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
          >
            <CircleHelp className="h-[18px] w-[18px] shrink-0" />
            {!currentCollapsed && <span>{t("help")}</span>}
          </button>

          <div className="h-px bg-sidebar-accent" />

          <div
            className={cn(
              "flex px-2",
              currentCollapsed
                ? "flex-col items-center gap-2 py-1"
                : "h-10 items-center gap-2.5",
            )}
          >
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  aria-label={
                    currentCollapsed ? displayName : `${displayName} menu`
                  }
                  title={displayName}
                  className={cn(
                    "flex items-center rounded-md outline-none",
                    currentCollapsed
                      ? "justify-center"
                      : "min-w-0 flex-1 gap-2.5",
                  )}
                >
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-sidebar-accent">
                    <span className="text-[11px] font-semibold text-sidebar-accent-foreground">
                      {initials}
                    </span>
                  </div>
                  {!currentCollapsed && (
                    <span className="truncate text-[13px] text-sidebar-accent-foreground">
                      {displayName}
                    </span>
                  )}
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" className="w-56">
                <DropdownMenuLabel>{displayName}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem asChild>
                  <LocaleSwitcher />
                </DropdownMenuItem>
                <ThemeSelector variant="menu" />
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => {
                    if (isMobile) {
                      onMobileOpenChange(false);
                    }
                    startFederatedLogout("/login");
                  }}
                >
                  <LogOut className="mr-2 h-4 w-4" />
                  {tCommon("signOut")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            {!isMobile && (
              <button
                type="button"
                onClick={() => onCollapsedChange(!currentCollapsed)}
                aria-label={
                  currentCollapsed ? "Expand sidebar" : "Collapse sidebar"
                }
                title={currentCollapsed ? "Expand sidebar" : "Collapse sidebar"}
                className="shrink-0 text-sidebar-foreground transition-colors hover:text-sidebar-accent-foreground"
              >
                {currentCollapsed ? (
                  <PanelLeft className="h-4 w-4" />
                ) : (
                  <PanelLeftClose className="h-4 w-4" />
                )}
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }

  return (
    <>
      <Sheet open={mobileOpen} onOpenChange={onMobileOpenChange}>
        <SheetContent
          side="left"
          className="w-[18rem] border-r border-sidebar-accent bg-sidebar p-0 text-sidebar-foreground md:hidden"
        >
          <SheetHeader className="sr-only">
            <SheetTitle>Navigation</SheetTitle>
          </SheetHeader>
          {renderSidebarContent(false, true)}
        </SheetContent>
      </Sheet>

      <aside
        className={cn(
          "hidden min-h-screen flex-col justify-between bg-sidebar text-sidebar-foreground transition-[width] duration-200 md:flex",
          collapsed ? "w-16" : "w-60",
        )}
      >
        {renderSidebarContent(collapsed, false)}
      </aside>
    </>
  );
}

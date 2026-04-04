"use client";

import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  ChevronDown,
  Globe,
  Building2,
  Layers,
  ArrowLeft,
  Search,
  Loader2,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useScope } from "@/hooks/use-scope";
import {
  usePaginatedTenants,
  usePaginatedWorkspaces,
} from "@/hooks/use-scope-data";
import { cn } from "@/lib/utils";
import { setLastWorkspacePath } from "@/hooks/use-last-workspace";
import { useOnScopeSwitcherOpen } from "@/lib/scope-switcher-events";
import { SYSTEM_WORKSPACE_CODE, type Tenant, type Workspace } from "@/types/api";

const WORKSPACE_PATH_RE = /^\/t\/[^/]+\/w\/[^/]+/;

interface ScopeSwitcherProps {
  collapsed?: boolean;
}

type ViewMode = "tenants" | "workspaces";

export function ScopeSwitcher({ collapsed }: ScopeSwitcherProps) {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const { level, tenantCode, workspaceCode } = useScope();
  const tScope = useTranslations("scopeSwitcher");

  // View state
  const [view, setView] = useState<ViewMode>("tenants");
  const [selectedTenant, setSelectedTenant] = useState<{
    code: string;
    name: string;
  } | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen);
    if (!nextOpen) {
      return;
    }

    setSearchInput("");
    setDebouncedSearch("");
    if (tenantCode) {
      setView("workspaces");
      setSelectedTenant({ code: tenantCode, name: tenantCode });
      return;
    }

    setView("tenants");
    setSelectedTenant(null);
  }

  // Trigger label/icon/color
  const isSystemWorkspace =
    level === "workspace" && workspaceCode === SYSTEM_WORKSPACE_CODE;

  const scopeLabel =
    level === "global"
      ? tScope("globalScope")
      : isSystemWorkspace
        ? tenantCode ?? "Tenant"
        : workspaceCode ?? "Workspace";

  const ScopeIcon =
    level === "global"
      ? Globe
      : isSystemWorkspace
        ? Building2
        : Layers;

  const scopeIconColor =
    level === "global"
      ? "text-scope-global"
      : isSystemWorkspace
        ? "text-scope-system"
        : "text-scope-workspace";

  function navigateTo(path: string) {
    if (WORKSPACE_PATH_RE.test(path)) {
      setLastWorkspacePath(path);
    }
    setOpen(false);
    router.push(path);
  }

  function handleTenantClick(tenant: Tenant) {
    setSelectedTenant({ code: tenant.code, name: tenant.name });
    setView("workspaces");
    setSearchInput("");
    setDebouncedSearch("");
  }

  function handleBack() {
    setView("tenants");
    setSelectedTenant(null);
    setSearchInput("");
    setDebouncedSearch("");
  }

  const handleScopeSwitcherOpen = useCallback((payload: { view: "tenants" | "workspaces"; tenantCode?: string; tenantName?: string }) => {
    setSearchInput("");
    setDebouncedSearch("");
    if (payload.view === "workspaces" && payload.tenantCode) {
      setSelectedTenant({
        code: payload.tenantCode,
        name: payload.tenantName ?? payload.tenantCode,
      });
      setView("workspaces");
    } else {
      setSelectedTenant(null);
      setView("tenants");
    }
    setOpen(true);
  }, []);

  useOnScopeSwitcherOpen(handleScopeSwitcherOpen);

  return (
    <>
      <button
        onClick={() => handleOpenChange(true)}
        className={cn(
          "mx-3 flex h-10 items-center gap-2 rounded-md border border-sidebar-border px-3 transition-colors hover:bg-sidebar-accent",
          collapsed && "justify-center px-0"
        )}
      >
        <ScopeIcon
          className={cn("h-3.5 w-3.5 shrink-0", scopeIconColor)}
        />
        {!collapsed && (
          <>
            <span className="flex-1 truncate text-left text-xs font-medium text-sidebar-accent-foreground">
              {scopeLabel}
            </span>
            <ChevronDown className="h-3.5 w-3.5 text-sidebar-foreground shrink-0" />
          </>
        )}
      </button>

      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className="sm:max-w-md p-0 gap-0 overflow-hidden">
          <DialogTitle className="sr-only">Select scope</DialogTitle>

          {/* Header */}
          <div className="flex items-center gap-2 px-4 pt-4 pb-2">
            {view === "workspaces" && (
              <button
                onClick={handleBack}
                className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors shrink-0"
              >
                <ArrowLeft className="h-4 w-4" />
              </button>
            )}
            <h3 className="text-sm font-medium truncate">
              {view === "tenants"
                ? "Select scope"
                : selectedTenant?.name ?? "Workspaces"}
            </h3>
          </div>

          {/* Search input */}
          <div className="px-4 pb-2">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder={
                  view === "tenants"
                    ? tScope("searchTenants")
                    : tScope("searchWorkspaces")
                }
                className="pl-8 h-9 text-sm"
              />
            </div>
          </div>

          {/* Scrollable list with slide transition */}
          <div className="h-[320px] overflow-hidden border-t">
            <div
              className={cn(
                "flex h-full transition-transform duration-200 ease-in-out",
                view === "workspaces" && "-translate-x-1/2"
              )}
              style={{ width: "200%" }}
            >
              <div className="w-1/2 overflow-y-auto h-full">
                <TenantsView
                  search={debouncedSearch}
                  onSelectGlobal={() => navigateTo("/global")}
                  onSelectTenant={handleTenantClick}
                  currentLevel={level}
                />
              </div>
              <div className="w-1/2 overflow-y-auto h-full">
                {selectedTenant && (
                  <WorkspacesView
                    tenantCode={selectedTenant.code}
                    search={debouncedSearch}
                    currentLevel={level}
                    onSelectWorkspace={(w) =>
                      navigateTo(
                        `/t/${selectedTenant.code}/w/${w.code}`
                      )
                    }
                    onManageWorkspaces={() =>
                      navigateTo(`/t/${selectedTenant.code}/w/${SYSTEM_WORKSPACE_CODE}/workspaces`)
                    }
                    currentWorkspaceCode={workspaceCode}
                  />
                )}
              </div>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

/* ── Tenants view ──────────────────────────────────── */

function TenantsView({
  search,
  onSelectGlobal,
  onSelectTenant,
  currentLevel,
}: {
  search: string;
  onSelectGlobal: () => void;
  onSelectTenant: (t: Tenant) => void;
  currentLevel: string;
}) {
  const tScope = useTranslations("scopeSwitcher");
  const { data, hasNextPage, fetchNextPage, isFetchingNextPage, isLoading } =
    usePaginatedTenants(search);

  const tenants = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data]
  );

  const sentinelRef = useInfiniteScroll(
    hasNextPage ?? false,
    isFetchingNextPage,
    fetchNextPage
  );

  return (
    <div className="flex flex-col">
      {/* Global option — only show when not searching */}
      {!search && (
        <button
          onClick={onSelectGlobal}
          className={cn(
            "flex items-center gap-3 px-4 py-2.5 text-sm hover:bg-muted/50 transition-colors text-left",
            currentLevel === "global" && "bg-muted/50"
          )}
        >
          <Globe className="h-4 w-4 text-scope-global shrink-0" />
          <span className="font-medium">{tScope("globalScope")}</span>
        </button>
      )}

      {/* Separator */}
      {!search && tenants.length > 0 && (
        <div className="border-b mx-4 my-1" />
      )}

      {/* Tenant list */}
      {isLoading ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      ) : tenants.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-8">
          {search ? tScope("noTenantsFound") : tScope("noTenantsAvailable")}
        </p>
      ) : (
        tenants.map((t) => (
          <button
            key={t.id}
            onClick={() => onSelectTenant(t)}
            className="flex items-center gap-3 px-4 py-2.5 text-sm hover:bg-muted/50 transition-colors text-left"
          >
            <Building2 className="h-4 w-4 text-scope-system shrink-0" />
            <div className="flex flex-col min-w-0">
              <span className="font-medium truncate">{t.name}</span>
              <span className="text-xs text-muted-foreground truncate">
                {t.code}
              </span>
            </div>
          </button>
        ))
      )}

      {/* Infinite scroll sentinel */}
      <div ref={sentinelRef} className="h-1" />
      {isFetchingNextPage && (
        <div className="flex items-center justify-center py-2">
          <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
        </div>
      )}
    </div>
  );
}

/* ── Workspaces view ───────────────────────────────── */

function WorkspacesView({
  tenantCode,
  search,
  currentLevel,
  onSelectWorkspace,
  onManageWorkspaces,
  currentWorkspaceCode,
}: {
  tenantCode: string;
  search: string;
  currentLevel: string;
  onSelectWorkspace: (w: Workspace) => void;
  onManageWorkspaces: () => void;
  currentWorkspaceCode?: string;
}) {
  const tScope = useTranslations("scopeSwitcher");
  const { data, hasNextPage, fetchNextPage, isFetchingNextPage, isLoading } =
    usePaginatedWorkspaces(tenantCode, search);

  const workspaces = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data]
  );

  const sentinelRef = useInfiniteScroll(
    hasNextPage ?? false,
    isFetchingNextPage,
    fetchNextPage
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (workspaces.length === 0) {
    return (
      <p className="text-sm text-muted-foreground text-center py-8">
        {search ? tScope("noWorkspacesFound") : tScope("noWorkspacesAvailable")}
      </p>
    );
  }

  return (
    <div className="flex flex-col">
      {(currentLevel !== "workspace" ||
        currentWorkspaceCode === SYSTEM_WORKSPACE_CODE) && (
        <div className="border-b px-4 py-3">
          <Button
            variant="outline"
            className="w-full justify-center"
            onClick={onManageWorkspaces}
          >
            {tScope("manageWorkspaces")}
          </Button>
        </div>
      )}
      {workspaces.map((w) => (
        <button
          key={w.id}
          onClick={() => onSelectWorkspace(w)}
          className={cn(
            "flex items-center gap-3 px-4 py-2.5 text-sm hover:bg-muted/50 transition-colors text-left",
            currentWorkspaceCode === w.code && "bg-muted/50"
          )}
        >
          <Layers className="h-4 w-4 text-scope-workspace shrink-0" />
          <div className="flex flex-col min-w-0">
            <span className="font-medium truncate">{w.name}</span>
            <span className="text-xs text-muted-foreground truncate">
              {w.code}
            </span>
          </div>
        </button>
      ))}

      {/* Infinite scroll sentinel */}
      <div ref={sentinelRef} className="h-1" />
      {isFetchingNextPage && (
        <div className="flex items-center justify-center py-2">
          <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
        </div>
      )}
    </div>
  );
}

/* ── Infinite scroll hook ──────────────────────────── */

function useInfiniteScroll(
  hasNextPage: boolean,
  isFetchingNextPage: boolean,
  fetchNextPage: () => void
) {
  const sentinelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasNextPage && !isFetchingNextPage) {
          fetchNextPage();
        }
      },
      { threshold: 0.1 }
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  return sentinelRef;
}

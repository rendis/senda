"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import { useRouter } from "next/navigation";
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
import { Input } from "@/components/ui/input";
import { useScope } from "@/hooks/use-scope";
import {
  usePaginatedTenants,
  usePaginatedWorkspaces,
} from "@/hooks/use-scope-data";
import { cn } from "@/lib/utils";
import type { Tenant, Workspace } from "@/types/api";

interface ScopeSwitcherProps {
  collapsed?: boolean;
}

type ViewMode = "tenants" | "workspaces";

export function ScopeSwitcher({ collapsed }: ScopeSwitcherProps) {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const { level, tenantCode, workspaceCode } = useScope();

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
  const scopeLabel =
    level === "global"
      ? "Global"
      : level === "workspace"
        ? workspaceCode ?? "Workspace"
        : tenantCode ?? "Tenant";

  const ScopeIcon =
    level === "global"
      ? Globe
      : level === "workspace"
        ? Layers
        : Building2;

  const scopeIconColor =
    level === "global"
      ? "text-scope-global"
      : level === "workspace"
        ? "text-scope-workspace"
        : "text-scope-system";

  function navigateTo(path: string) {
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

  return (
    <>
      <button
        onClick={() => handleOpenChange(true)}
        className={cn(
          "flex items-center gap-2 mx-3 px-3 h-10 rounded-md border border-[#334155] hover:bg-sidebar-accent transition-colors",
          collapsed && "justify-center px-0"
        )}
      >
        <ScopeIcon
          className={cn("h-3.5 w-3.5 shrink-0", scopeIconColor)}
        />
        {!collapsed && (
          <>
            <span className="text-xs font-medium text-white truncate flex-1 text-left">
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
                    ? "Search tenants..."
                    : "Search workspaces..."
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
                    onSelectWorkspace={(w) =>
                      navigateTo(
                        `/t/${selectedTenant.code}/w/${w.code}`
                      )
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
          <span className="font-medium">Global</span>
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
          {search ? "No tenants found" : "No tenants available"}
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
  onSelectWorkspace,
  currentWorkspaceCode,
}: {
  tenantCode: string;
  search: string;
  onSelectWorkspace: (w: Workspace) => void;
  currentWorkspaceCode?: string;
}) {
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
        {search ? "No workspaces found" : "No workspaces available"}
      </p>
    );
  }

  return (
    <div className="flex flex-col">
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

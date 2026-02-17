"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Box, ChevronDown, Globe, Building2, Layers } from "lucide-react";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import { useScope } from "@/hooks/use-scope";
import { useTenantsQuery, useWorkspacesQuery } from "@/hooks/use-scope-data";
import { cn } from "@/lib/utils";

interface ScopeSwitcherProps {
  collapsed?: boolean;
}

export function ScopeSwitcher({ collapsed }: ScopeSwitcherProps) {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const { level, tenantCode, workspaceCode } = useScope();
  const { data: tenants = [] } = useTenantsQuery();
  const { data: workspaces = [] } = useWorkspacesQuery(tenantCode);

  const scopeLabel =
    level === "global"
      ? "Global"
      : level === "workspace"
        ? workspaceCode ?? "Workspace"
        : tenantCode ?? "Tenant";

  function navigateTo(path: string) {
    setOpen(false);
    router.push(path);
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className={cn(
            "flex items-center gap-2 mx-3 px-3 h-10 rounded-md border border-[#334155] hover:bg-sidebar-accent transition-colors",
            collapsed && "justify-center px-0"
          )}
        >
          <Box className="h-3.5 w-3.5 text-primary shrink-0" />
          {!collapsed && (
            <>
              <span className="text-xs font-medium text-white truncate flex-1 text-left">
                {scopeLabel}
              </span>
              <ChevronDown className="h-3.5 w-3.5 text-sidebar-foreground shrink-0" />
            </>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-56 p-0" align="start" side="right">
        <Command>
          <CommandInput placeholder="Search..." />
          <CommandList>
            <CommandEmpty>No results.</CommandEmpty>

            {/* Global scope */}
            <CommandGroup heading="Scope">
              <CommandItem onSelect={() => navigateTo("/global")}>
                <Globe className="mr-2 h-4 w-4 text-scope-global" />
                <span>Global</span>
              </CommandItem>
            </CommandGroup>

            {/* Tenants */}
            {tenants.length > 0 && (
              <>
                <CommandSeparator />
                <CommandGroup heading="Tenants">
                  {tenants.map((t) => (
                    <CommandItem
                      key={t.id}
                      onSelect={() => navigateTo(`/t/${t.code}`)}
                    >
                      <Building2 className="mr-2 h-4 w-4 text-scope-system" />
                      <span>{t.name}</span>
                    </CommandItem>
                  ))}
                </CommandGroup>
              </>
            )}

            {/* Workspaces for current tenant */}
            {workspaces.length > 0 && tenantCode && (
              <>
                <CommandSeparator />
                <CommandGroup heading="Workspaces">
                  {workspaces.map((w) => (
                    <CommandItem
                      key={w.id}
                      onSelect={() =>
                        navigateTo(`/t/${tenantCode}/w/${w.code}`)
                      }
                    >
                      <Layers className="mr-2 h-4 w-4 text-scope-workspace" />
                      <span>{w.name}</span>
                    </CommandItem>
                  ))}
                </CommandGroup>
              </>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

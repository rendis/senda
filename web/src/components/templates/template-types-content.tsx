"use client";

import { useState, useMemo } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { useRouter } from "next/navigation";
import {
  MoreHorizontal,
  Plus,
  Search,
  FileType,
  TriangleAlert,
} from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import { useTemplateTypes, useCreateTemplateType } from "@/hooks/use-template-types";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { FormDialog } from "@/components/shared/form-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { TemplateType } from "@/types/templates";
import { toast } from "sonner";

export function TemplateTypesContent() {
  const scope = useScope();

  if (scope.level === "tenant") {
    return (
      <EmptyState
        icon={FileType}
        title="Select a workspace"
        description="Template types are workspace-scoped. Select a workspace from the sidebar to manage template types."
      />
    );
  }

  return <TemplateTypesTable />;
}

function TemplateTypesTable() {
  const router = useRouter();
  const scope = useScope();
  const scopedPath = useScopedPath();
  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useTemplateTypes(scopedPath);
  const createMutation = useCreateTemplateType(scopedPath);

  const [search, setSearch] = useState("");
  const [newSlug, setNewSlug] = useState("");
  const [newName, setNewName] = useState("");

  const allItems = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data]
  );

  const filtered = useMemo(() => {
    if (!search) return allItems;
    const q = search.toLowerCase();
    return allItems.filter(
      (t) =>
        t.slug.toLowerCase().includes(q) ||
        t.name.toLowerCase().includes(q)
    );
  }, [allItems, search]);

  function buildTypePath(slug: string) {
    switch (scope.level) {
      case "global":
        return `/global/templates/${slug}`;
      case "tenant":
        return `/t/${scope.tenantCode}/templates/${slug}`;
      case "workspace":
        return `/t/${scope.tenantCode}/w/${scope.workspaceCode}/templates/${slug}`;
    }
  }

  const columns: ColumnDef<TemplateType>[] = [
    {
      accessorKey: "slug",
      header: "SLUG",
      cell: ({ row }) => (
        <button
          className="font-mono text-sm font-medium text-primary hover:underline"
          onClick={() => router.push(buildTypePath(row.original.slug))}
        >
          {row.original.slug}
        </button>
      ),
    },
    {
      accessorKey: "name",
      header: "NAME",
      cell: ({ row }) => (
        <span className="text-sm text-foreground">{row.original.name}</span>
      ),
    },
    {
      accessorKey: "adapter_id",
      header: "ADAPTER",
      cell: ({ row }) =>
        row.original.adapter_id ? (
          <span className="font-mono text-xs text-foreground">
            {row.original.adapter_id}
          </span>
        ) : (
          <span className="inline-flex items-center gap-1 text-status-complained text-xs">
            <TriangleAlert className="h-3.5 w-3.5" />
            No adapter
          </span>
        ),
    },
    {
      accessorKey: "scope_level",
      header: "SCOPE",
      cell: ({ row }) => (
        <ScopeIndicator scope={row.original.scope_level} />
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              onClick={() => router.push(buildTypePath(row.original.slug))}
            >
              View templates
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  async function handleCreateType() {
    if (!newSlug.trim() || !newName.trim()) return;
    try {
      await createMutation.mutateAsync({
        slug: newSlug.trim(),
        name: newName.trim(),
      });
      toast.success("Template type created");
      setNewSlug("");
      setNewName("");
    } catch {
      toast.error("Failed to create template type");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div className="relative w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search template types..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 h-9"
          />
        </div>
        <FormDialog
          trigger={
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              New Template Type
            </Button>
          }
          title="Create Template Type"
          description="Define a new template type for this scope."
          submitLabel="Create"
          onSubmit={handleCreateType}
        >
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="tt-slug">Slug</Label>
              <Input
                id="tt-slug"
                placeholder="welcome-email"
                value={newSlug}
                onChange={(e) => setNewSlug(e.target.value)}
                className="font-mono"
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="tt-name">Name</Label>
              <Input
                id="tt-name"
                placeholder="Welcome Email"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
              />
            </div>
          </div>
        </FormDialog>
      </div>

      <DataTable
        columns={columns}
        data={filtered}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={FileType}
            title="No template types"
            description="Create your first template type to organize your email templates."
            action={
              <Button variant="outline" size="sm">
                <Plus className="h-4 w-4 mr-2" />
                Create Template Type
              </Button>
            }
          />
        }
      />
    </div>
  );
}

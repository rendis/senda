"use client";

import { useState } from "react";
import { Plug, Plus, MoreHorizontal, Check, Trash2, Zap } from "lucide-react";
import { useScopedPath } from "@/hooks/use-scope";
import {
  useAdapterList,
  useCreateAdapter,
  useDeleteAdapter,
  useTestAdapterConnection,
} from "@/hooks/use-adapters";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { AdapterTypeBadge } from "./adapter-type-badge";
import { AdapterForm } from "./adapter-form";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ColumnDef } from "@tanstack/react-table";
import type { Adapter, CreateAdapterRequest } from "@/types/adapters";

export function AdaptersContent() {
  const scopedPath = useScopedPath();
  const [search, setSearch] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<Adapter | null>(null);

  const {
    data: listData,
    isLoading,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useAdapterList(scopedPath);

  const createAdapter = useCreateAdapter(scopedPath);
  const deleteAdapter = useDeleteAdapter(scopedPath);

  const allItems = listData?.pages.flatMap((p) => p.items) ?? [];
  const items = search
    ? allItems.filter((a) =>
        a.name.toLowerCase().includes(search.toLowerCase())
      )
    : allItems;

  async function handleCreate(data: CreateAdapterRequest) {
    await createAdapter.mutateAsync(data);
  }

  const columns: ColumnDef<Adapter, unknown>[] = [
    {
      accessorKey: "name",
      header: "NAME",
      cell: ({ row }) => (
        <span className="font-medium text-[13px] text-foreground">
          {row.original.name}
        </span>
      ),
    },
    {
      accessorKey: "adapter_type",
      header: "TYPE",
      cell: ({ row }) => <AdapterTypeBadge type={row.original.adapter_type} />,
    },
    {
      id: "scope",
      header: "SCOPE",
      cell: () => <ScopeIndicator scope="workspace" />,
    },
    {
      accessorKey: "is_default",
      header: "DEFAULT",
      cell: ({ row }) =>
        row.original.is_default ? (
          <Check className="h-4 w-4 text-primary" />
        ) : (
          <span className="text-muted-foreground">&mdash;</span>
        ),
    },
    {
      accessorKey: "rate_limit_per_second",
      header: "RATE LIMIT",
      cell: ({ row }) => (
        <span className="font-mono text-[13px]">
          {row.original.rate_limit_per_second
            ? `${row.original.rate_limit_per_second}/sec`
            : "\u2014"}
        </span>
      ),
    },
    {
      id: "actions",
      enableSorting: false,
      cell: ({ row }) => <AdapterActions adapter={row.original} onDelete={setDeleteTarget} />,
    },
  ];

  return (
    <div className="flex flex-col gap-6">
      {/* Toolbar */}
      <div className="flex items-center justify-between">
        <Input
          placeholder="Search adapters..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-[280px]"
        />
        <AdapterForm
          trigger={
            <Button className="gap-2">
              <Plus className="h-4 w-4" />
              New Adapter
            </Button>
          }
          onSubmit={handleCreate}
        />
      </div>

      {/* Table */}
      <DataTable
        columns={columns}
        data={items}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={Plug}
            title="No adapters configured"
            description="Add an email adapter (SES or Gmail) to start sending emails from this scope."
            action={
              <AdapterForm
                trigger={
                  <Button className="gap-2">
                    <Plus className="h-4 w-4" />
                    Add Adapter
                  </Button>
                }
                onSubmit={handleCreate}
              />
            }
          />
        }
      />

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Adapter"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) {
            deleteAdapter.mutate(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        loading={deleteAdapter.isPending}
      />
    </div>
  );
}

function AdapterActions({
  adapter,
  onDelete,
}: {
  adapter: Adapter;
  onDelete: (a: Adapter) => void;
}) {
  const scopedPath = useScopedPath();
  const testConnection = useTestAdapterConnection(scopedPath, adapter.id);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
          <MoreHorizontal className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          onClick={() => testConnection.mutate()}
          disabled={testConnection.isPending}
        >
          <Zap className="mr-2 h-4 w-4" />
          Test Connection
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => onDelete(adapter)}
          className="text-destructive"
        >
          <Trash2 className="mr-2 h-4 w-4" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

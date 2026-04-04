"use client";

import { useState } from "react";
import { useMinimumLoading } from "@/hooks/use-minimum-loading";
import { Database, ArrowLeft, Plus, Eye, Trash2 } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useInjectorList,
  useInjectorDetail,
  useSetInjectorValues,
  useDeleteInjectorOverride,
  useCreateInjector,
  useDeleteInjector,
} from "@/hooks/use-injectors";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { InjectorFieldEditor } from "./injector-field-editor";
import { InjectorForm } from "./injector-form";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ColumnDef } from "@tanstack/react-table";
import type { InjectorDefinition, CreateInjectorRequest } from "@/types/injectors";

export function InjectorsContent() {
  return <InjectorsTable />;
}

function InjectorsTable() {
  const scope = useScope();
  const scopedPath = useScopedPath();
  const [selectedInjector, setSelectedInjector] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  const {
    data: listData,
    isLoading: listLoading,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useInjectorList(scopedPath);

  const createInjector = useCreateInjector(scopedPath);
  const deleteInjector = useDeleteInjector(scopedPath);
  const [deleteTarget, setDeleteTarget] = useState<InjectorDefinition | null>(null);

  const {
    data: detail,
    isLoading: rawDetailLoading,
  } = useInjectorDetail(scopedPath, selectedInjector ?? "");
  const detailLoading = useMinimumLoading(rawDetailLoading);

  const setValues = useSetInjectorValues(scopedPath, selectedInjector ?? "");
  const deleteOverride = useDeleteInjectorOverride(
    scopedPath,
    selectedInjector ?? ""
  );

  const allItems = listData?.pages.flatMap((p) => p.items) ?? [];
  const items = search
    ? allItems.filter((a) =>
        a.name.toLowerCase().includes(search.toLowerCase())
      )
    : allItems;

  async function handleCreate(data: CreateInjectorRequest) {
    await createInjector.mutateAsync(data);
  }

  // Detail view
  if (selectedInjector) {
    return (
      <div className="flex flex-col gap-6">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setSelectedInjector(null)}
          className="w-fit gap-2"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Injectors
        </Button>

        {detailLoading ? (
          <InjectorDetailSkeleton />
        ) : detail ? (
          <div className="animate-in fade-in duration-300">
            <div className="flex items-center gap-4">
              <h2
                className="text-xl font-semibold"
                style={{ letterSpacing: "-1px" }}
              >
                {detail.name}
              </h2>
              <ScopeIndicator scope={detail.scope_level} />
            </div>
            {detail.description && (
              <p className="text-sm text-muted-foreground">
                {detail.description}
              </p>
            )}
            <div className="flex flex-col gap-4 mt-4">
              {(detail.fields ?? []).length === 0 && (
                <p className="text-sm text-muted-foreground italic">No fields defined for this injector.</p>
              )}
              {(detail.fields ?? [])
                .sort((a, b) => a.position - b.position)
                .map((field) => {
                  const resolution = detail.values?.[field.field_name];
                  return (
                    <InjectorFieldEditor
                      key={field.field_name}
                      field={field}
                      resolution={resolution ?? null}
                      currentScope={scope.level}
                      onSave={(fieldName, value) =>
                        setValues.mutate({ values: { [fieldName]: value } })
                      }
                      onDeleteOverride={(fieldName) =>
                        deleteOverride.mutate(fieldName)
                      }
                      saving={setValues.isPending || deleteOverride.isPending}
                    />
                  );
                })}
            </div>
          </div>
        ) : null}
      </div>
    );
  }

  // List view with clickable rows
  const listColumns: ColumnDef<InjectorDefinition, unknown>[] = [
    {
      accessorKey: "name",
      header: "NAME",
      cell: ({ row }) => (
        <button
          onClick={() => setSelectedInjector(row.original.name)}
          className="font-mono text-[13px] font-medium text-foreground hover:text-primary transition-colors text-left"
        >
          {row.original.name}
        </button>
      ),
    },
    {
      accessorKey: "description",
      header: "DESCRIPTION",
      cell: ({ row }) => (
        <span className="text-sm text-muted-foreground">
          {row.original.description ?? "\u2014"}
        </span>
      ),
    },
    {
      accessorKey: "scope_level",
      header: "SCOPE",
      cell: ({ row }) => <ScopeIndicator scope={row.original.scope_level} />,
    },
    {
      accessorKey: "fields",
      header: "FIELDS",
      enableSorting: false,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {row.original.fields?.length ?? 0} fields
        </span>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex items-center justify-end gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setSelectedInjector(row.original.name)}>
                <Eye className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>View details</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => setDeleteTarget(row.original)}>
                <Trash2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Delete</TooltipContent>
          </Tooltip>
        </div>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-6">
      {/* Toolbar */}
      <div className="flex items-center justify-between">
        <Input
          placeholder="Search injectors..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-[280px]"
        />
        <InjectorForm
          trigger={
            <Button className="gap-2">
              <Plus className="h-4 w-4" />
              New Injector
            </Button>
          }
          onSubmit={handleCreate}
        />
      </div>

      {/* Table */}
      <DataTable
        columns={listColumns}
        data={items}
        loading={listLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={Database}
            title="No injectors defined"
            description="Injectors provide dynamic values to your templates."
            action={
              <InjectorForm
                trigger={
                  <Button className="gap-2">
                    <Plus className="h-4 w-4" />
                    Add Injector
                  </Button>
                }
                onSubmit={handleCreate}
              />
            }
          />
        }
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Injector"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? Templates using this injector will no longer receive its values.`}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) {
            deleteInjector.mutate(deleteTarget.name);
            setDeleteTarget(null);
          }
        }}
        loading={deleteInjector.isPending}
      />
    </div>
  );
}

function InjectorDetailSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="h-4 w-96" />
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}

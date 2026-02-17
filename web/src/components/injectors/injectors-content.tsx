"use client";

import { useState } from "react";
import { Database, ArrowLeft } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useInjectorList,
  useInjectorDetail,
  useSetInjectorValues,
  useDeleteInjectorOverride,
} from "@/hooks/use-injectors";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { InjectorFieldEditor } from "./injector-field-editor";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { ColumnDef } from "@tanstack/react-table";
import type { InjectorDefinition } from "@/types/injectors";

export function InjectorsContent() {
  const scope = useScope();
  const scopedPath = useScopedPath();
  const [selectedInjector, setSelectedInjector] = useState<string | null>(null);

  const {
    data: listData,
    isLoading: listLoading,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useInjectorList(scopedPath);

  const {
    data: detail,
    isLoading: detailLoading,
  } = useInjectorDetail(scopedPath, selectedInjector ?? "");

  const setValues = useSetInjectorValues(scopedPath, selectedInjector ?? "");
  const deleteOverride = useDeleteInjectorOverride(
    scopedPath,
    selectedInjector ?? ""
  );

  const items = listData?.pages.flatMap((p) => p.items) ?? [];

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
          <>
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
            <div className="flex flex-col gap-4">
              {detail.fields
                .sort((a, b) => a.position - b.position)
                .map((field) => {
                  const resolution = detail.values[field.field_name];
                  if (!resolution) return null;
                  return (
                    <InjectorFieldEditor
                      key={field.field_name}
                      field={field}
                      resolution={resolution}
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
          </>
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
          {row.original.fields.length} fields
        </span>
      ),
    },
  ];

  return (
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
          description="Injectors provide dynamic values to your templates. They are configured at the global level."
        />
      }
    />
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

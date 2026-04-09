"use client";

import { useState } from "react";
import {
  Database,
  ArrowLeft,
  Pencil,
  Plus,
  Eye,
  Trash2,
  Lock,
  RefreshCcw,
} from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useInjectorList,
  useInjectorDetail,
  useCreateInjector,
  useDeleteInjector,
  useUpdateInjector,
} from "@/hooks/use-injectors";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { InjectorFieldCard } from "@/components/injectors/injector-field-card";
import { InjectorForm } from "./injector-form";
import {
  canEditInjectorSchema,
  resolveUpdatedInjectorSelection,
  supportsInjectorManagementScope,
} from "@/components/injectors/injector-form-model";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ColumnDef } from "@tanstack/react-table";
import type {
  InjectorDefinition,
  CreateInjectorRequest,
  UpdateInjectorRequest,
} from "@/types/injectors";

export function InjectorsContent() {
  const scope = useScope();

  if (!supportsInjectorManagementScope(scope.level)) {
    return (
      <EmptyState
        icon={Database}
        title="Injectors are managed per writable scope"
        description="Choose the global catalog or a workspace to define and edit injector schemas."
      />
    );
  }

  return <InjectorsTable />;
}

function InjectorsTable() {
  const scope = useScope();
  const scopedPath = useScopedPath();
  const [selectedInjector, setSelectedInjector] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [editingOpen, setEditingOpen] = useState(false);

  const { data: listData, isLoading: listLoading } = useInjectorList(scopedPath);
  const createInjector = useCreateInjector(scopedPath);
  const updateInjector = useUpdateInjector(scopedPath);
  const deleteInjector = useDeleteInjector(scopedPath);
  const [deleteTarget, setDeleteTarget] = useState<InjectorDefinition | null>(null);

  const { data: detail, isLoading: detailLoading } = useInjectorDetail(
    scopedPath,
    selectedInjector ?? "",
    !!selectedInjector,
  );

  const allItems = listData?.items ?? [];
  const items = search
    ? allItems.filter((item) => item.name.toLowerCase().includes(search.toLowerCase()))
    : allItems;

  async function handleCreate(data: CreateInjectorRequest) {
    await createInjector.mutateAsync(data);
  }

  async function handleUpdate(data: UpdateInjectorRequest) {
    if (!detail) {
      return;
    }

    const updated = await updateInjector.mutateAsync({
      currentName: detail.name,
      data,
    });

    setSelectedInjector(resolveUpdatedInjectorSelection(updated));
    setEditingOpen(false);
  }

  const selectedCanEdit = canEditInjectorSchema(scope.level, detail);

  if (selectedInjector) {
    return (
      <div className="flex flex-col gap-6">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            setEditingOpen(false);
            setSelectedInjector(null);
          }}
          className="w-fit gap-2"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Injectors
        </Button>

        {detailLoading ? (
          <InjectorDetailSkeleton />
        ) : detail ? (
          <div className="animate-in fade-in duration-300 flex flex-col gap-5">
            <div className="flex items-start justify-between gap-4">
              <div className="flex flex-col gap-2">
                <div className="flex items-center gap-2">
                  <h2 className="text-xl font-semibold" style={{ letterSpacing: "-1px" }}>
                    {detail.name}
                  </h2>
                  <Badge variant="outline">
                    {detail.workspace_id ? "Workspace" : "Global"}
                  </Badge>
                </div>
                {detail.description ? (
                  <p className="text-sm text-muted-foreground">{detail.description}</p>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    No description set for this injector yet.
                  </p>
                )}
              </div>

              {selectedCanEdit ? (
                <InjectorForm
                  key={`edit-${detail.name}`}
                  mode="edit"
                  injector={detail}
                  open={editingOpen}
                  onOpenChange={setEditingOpen}
                  onSubmit={handleUpdate}
                  trigger={
                    <Button className="gap-2">
                      <Pencil className="h-4 w-4" />
                      Edit injector
                    </Button>
                  }
                />
              ) : (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span>
                      <Button disabled className="gap-2">
                        <Pencil className="h-4 w-4" />
                        Edit injector
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>
                    Only injectors owned by the current writable scope can be edited.
                  </TooltipContent>
                </Tooltip>
              )}
            </div>

            <div className="rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground">
              Schema updates are replace-all: renames or type changes do not migrate
              existing template references, and all injector values for this definition are
              cleared when you save.
            </div>

            <div className="flex flex-col gap-4">
              {(detail.fields ?? []).length === 0 ? (
                <p className="text-sm text-muted-foreground italic">
                  No fields defined for this injector.
                </p>
              ) : (
                [...(detail.fields ?? [])]
                  .sort((a, b) => a.position - b.position)
                  .map((field) => (
                    <InjectorFieldCard
                      key={field.field_name}
                      title={field.field_name}
                      typeLabel={field.field_type}
                      description={field.description}
                    >
                      <div className="grid gap-3 text-sm text-muted-foreground md:grid-cols-2">
                        <FieldMeta
                          label="Mode"
                          value={
                            field.allow_overwrite ? "overwrite enabled" : "locked to default"
                          }
                        />
                        <FieldMeta
                          label="Position"
                          value={String(field.position + 1)}
                        />
                        <FieldMeta
                          label="Default"
                          value={formatDefaultValue(field.default_value)}
                        />
                        <FieldMeta label="Field type" value={field.field_type} />
                      </div>
                    </InjectorFieldCard>
                  ))
              )}
            </div>
          </div>
        ) : null}
      </div>
    );
  }

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
      id: "scope",
      header: "SCOPE",
      enableSorting: false,
      cell: ({ row }) => (
        <Badge variant="outline">{row.original.workspace_id ? "Workspace" : "Global"}</Badge>
      ),
    },
    {
      id: "defaults",
      header: "FIELDS",
      enableSorting: false,
      cell: ({ row }) => {
        const fieldCount = row.original.fields?.length ?? 0;
        const lockedCount =
          row.original.fields?.filter((field) => !field.allow_overwrite).length ?? 0;
        const overwriteCount = fieldCount - lockedCount;

        return (
          <div className="flex flex-col gap-1 text-xs text-muted-foreground">
            <span className="font-mono">{fieldCount} fields</span>
            <span className="flex items-center gap-3">
              <span className="inline-flex items-center gap-1">
                <Lock className="h-3 w-3" />
                {lockedCount}
              </span>
              <span className="inline-flex items-center gap-1">
                <RefreshCcw className="h-3 w-3" />
                {overwriteCount}
              </span>
            </span>
          </div>
        );
      },
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex items-center justify-end gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => setSelectedInjector(row.original.name)}
              >
                <Eye className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>View fields</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-destructive"
                onClick={() => setDeleteTarget(row.original)}
              >
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
      <div className="flex items-center justify-between gap-4">
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

      <DataTable
        columns={listColumns}
        data={items}
        loading={listLoading}
        emptyState={
          <EmptyState
            icon={Database}
            title="No injectors defined"
            description="Define the tokens your template builders can use and how each field resolves at runtime."
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
        onOpenChange={(nextOpen) => !nextOpen && setDeleteTarget(null)}
        title="Delete Injector"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? Templates using this injector will no longer see its fields in the builder.`}
        confirmLabel="Delete"
        onConfirm={async () => {
          if (!deleteTarget) {
            return;
          }

          await deleteInjector.mutateAsync(deleteTarget.name);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}

function FieldMeta({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs uppercase tracking-wide text-muted-foreground/80">{label}</span>
      <span className="font-medium text-foreground">{value}</span>
    </div>
  );
}

function formatDefaultValue(value: unknown): string {
  if (value == null || value === "") {
    return "∅";
  }

  if (typeof value === "string") {
    return value;
  }

  return JSON.stringify(value);
}

function InjectorDetailSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <Skeleton className="h-6 w-56" />
      <Skeleton className="h-4 w-80" />
      <Skeleton className="h-20 w-full" />
      <Skeleton className="h-44 w-full" />
      <Skeleton className="h-44 w-full" />
    </div>
  );
}

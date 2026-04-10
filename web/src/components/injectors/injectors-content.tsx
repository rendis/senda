"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
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
  getInjectorManagementState,
  resolveResourceDisplayScope,
} from "@/lib/workspace-resource-policies";
import {
  useInjectorList,
  useInjectorDetail,
  useCreateInjector,
  useDeleteInjector,
  useUpdateInjector,
} from "@/hooks/use-injectors";
import { useResolvedWorkspacePolicies } from "@/hooks/use-settings";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ResourceStateBadges } from "@/components/shared/resource-state-badges";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { InjectorFieldCard } from "@/components/injectors/injector-field-card";
import { InjectorForm } from "./injector-form";
import {
  canEditInjectorSchema,
  resolveUpdatedInjectorSelection,
  supportsInjectorManagementScope,
} from "@/components/injectors/injector-form-model";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  const t = useTranslations("injectorsPage");
  const scope = useScope();

  if (!supportsInjectorManagementScope(scope.level)) {
    return (
      <EmptyState
        icon={Database}
        title={t("unsupportedScopeTitle")}
        description={t("unsupportedScopeDescription")}
      />
    );
  }

  return <InjectorsTable />;
}

function InjectorsTable() {
  const t = useTranslations("injectorsPage");
  const tCommon = useTranslations("common");
  const scope = useScope();
  const scopedPath = useScopedPath();
  const workspacePolicies = useResolvedWorkspacePolicies(scope);
  const [selectedInjector, setSelectedInjector] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [editingOpen, setEditingOpen] = useState(false);

  const { data: listData, isLoading: listLoading } = useInjectorList(scopedPath, {
    includeInherited: scope.level === "workspace",
  });
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

  const selectedState = getInjectorManagementState(
    scope,
    detail,
    workspacePolicies.data,
  );
  const selectedCanEdit = canEditInjectorSchema(scope.level, detail) && selectedState.canEdit;
  const canCreateInjectors =
    scope.level !== "workspace" ||
    workspacePolicies.data?.allow_workspace_local_injectors === true;

  if (selectedInjector) {
    return (
      <InjectorDetailSection
        detail={detail}
        detailLoading={detailLoading}
        editingOpen={editingOpen}
        onOpenChange={setEditingOpen}
        onBack={() => {
          setEditingOpen(false);
          setSelectedInjector(null);
        }}
        onSubmit={handleUpdate}
        selectedCanEdit={selectedCanEdit}
        selectedState={selectedState}
      />
    );
  }

  const listColumns: ColumnDef<InjectorDefinition, unknown>[] = [
    {
      accessorKey: "name",
      header: t("columns.name"),
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
      header: tCommon("description"),
      cell: ({ row }) => (
        <span className="text-sm text-muted-foreground">
          {row.original.description ?? "\u2014"}
        </span>
      ),
    },
    {
      id: "scope",
      header: t("columns.scope"),
      enableSorting: false,
      cell: ({ row }) => (
        <div className="flex flex-wrap items-center gap-2">
          <ScopeIndicator scope={resolveResourceDisplayScope(row.original)} />
          <ResourceStateBadges
            badges={getInjectorManagementState(
              scope,
              row.original,
              workspacePolicies.data,
            ).badges}
          />
        </div>
      ),
    },
    {
      id: "defaults",
      header: t("columns.fields"),
      enableSorting: false,
      cell: ({ row }) => {
        const fieldCount = row.original.fields?.length ?? 0;
        const lockedCount =
          row.original.fields?.filter((field) => !field.allow_overwrite).length ?? 0;
        const overwriteCount = fieldCount - lockedCount;

        return (
          <div className="flex flex-col gap-1 text-xs text-muted-foreground">
            <span className="font-mono">{t("fieldCount", { count: fieldCount })}</span>
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
      cell: ({ row }) => {
        const itemState = getInjectorManagementState(
          scope,
          row.original,
          workspacePolicies.data,
        );
        const deleteReason =
          row.original.owner_scope === "local"
            ? t("localInjectorManagementDisabled")
            : t("defaultInjectorsReadonly");

        return (
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
              <TooltipContent>{t("viewFields")}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-destructive"
                    onClick={() => itemState.canDelete && setDeleteTarget(row.original)}
                    disabled={!itemState.canDelete}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>
                {itemState.canDelete ? tCommon("delete") : deleteReason}
              </TooltipContent>
            </Tooltip>
          </div>
        );
      },
    },
  ];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between gap-4">
        <Input
          placeholder={t("searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-[280px]"
        />
        <Tooltip>
          <TooltipTrigger asChild>
            <span>
              <Button
                className="gap-2"
                data-testid="injector-create-trigger"
                onClick={() => canCreateInjectors && setCreateOpen(true)}
                disabled={!canCreateInjectors}
              >
                <Plus className="h-4 w-4" />
                {t("newInjector")}
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>
            {canCreateInjectors
              ? t("createInScope")
              : t("localInjectorCreationDisabled")}
          </TooltipContent>
        </Tooltip>
      </div>

      <DataTable
        columns={listColumns}
        data={items}
        loading={listLoading}
        emptyState={
          <EmptyState
            icon={Database}
            title={t("empty.title")}
            description={t("empty.description")}
            action={
              <Button
                className="gap-2"
                data-testid="injector-empty-create-trigger"
                onClick={() => setCreateOpen(true)}
                disabled={!canCreateInjectors}
              >
                <Plus className="h-4 w-4" />
                {t("addInjector")}
              </Button>
            }
          />
        }
      />

      <InjectorForm
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSubmit={handleCreate}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(nextOpen) => !nextOpen && setDeleteTarget(null)}
        title={t("deleteDialog.title")}
        description={t("deleteDialog.description", { name: deleteTarget?.name ?? "" })}
        confirmLabel={tCommon("delete")}
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

function InjectorDetailSection({
  detail,
  detailLoading,
  editingOpen,
  onOpenChange,
  onBack,
  onSubmit,
  selectedCanEdit,
  selectedState,
}: {
  detail: InjectorDefinition | undefined;
  detailLoading: boolean;
  editingOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onBack: () => void;
  onSubmit: (data: UpdateInjectorRequest) => Promise<void>;
  selectedCanEdit: boolean;
  selectedState: ReturnType<typeof getInjectorManagementState>;
}) {
  const t = useTranslations("injectorsPage");

  return (
    <div className="flex flex-col gap-6">
      <Button
        variant="ghost"
        size="sm"
        onClick={onBack}
        className="w-fit gap-2"
      >
        <ArrowLeft className="h-4 w-4" />
        {t("backToInjectors")}
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
                <ScopeIndicator scope={resolveResourceDisplayScope(detail)} />
                <ResourceStateBadges badges={selectedState.badges} />
              </div>
              {detail.description ? (
                <p className="text-sm text-muted-foreground">{detail.description}</p>
              ) : (
                <p className="text-sm text-muted-foreground">{t("noDescription")}</p>
              )}
            </div>

            {selectedCanEdit ? (
              <InjectorForm
                key={`edit-${detail.name}`}
                mode="edit"
                injector={detail}
                open={editingOpen}
                onOpenChange={onOpenChange}
                onSubmit={onSubmit}
                trigger={
                  <Button className="gap-2">
                    <Pencil className="h-4 w-4" />
                    {t("editInjector")}
                  </Button>
                }
              />
            ) : (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span>
                    <Button disabled className="gap-2">
                      <Pencil className="h-4 w-4" />
                      {t("editInjector")}
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  {detail.owner_scope === "local"
                    ? t("localInjectorEditingDisabled")
                    : t("defaultInjectorsReadonly")}
                </TooltipContent>
              </Tooltip>
            )}
          </div>

          <div className="rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground">
            {t("replaceAllNotice")}
          </div>

          <div className="flex flex-col gap-4">
            {(detail.fields ?? []).length === 0 ? (
              <p className="text-sm text-muted-foreground italic">{t("noFields")}</p>
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
                        label={t("fieldMeta.mode")}
                        value={
                          field.allow_overwrite
                            ? t("fieldMeta.overwriteEnabled")
                            : t("fieldMeta.lockedToDefault")
                        }
                      />
                      <FieldMeta
                        label={t("fieldMeta.position")}
                        value={String(field.position + 1)}
                      />
                      <FieldMeta
                        label={t("fieldMeta.default")}
                        value={formatDefaultValue(field.default_value)}
                      />
                      <FieldMeta
                        label={t("fieldMeta.fieldType")}
                        value={field.field_type}
                      />
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

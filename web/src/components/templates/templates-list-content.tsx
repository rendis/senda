"use client";

import { useMemo } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { useRouter, useParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Plus, FileText, Pencil, Send, Trash2, Copy, GitFork, Lock } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  getTemplateCatalogState,
  getTemplateManagementState,
  resolveResourceDisplayScope,
} from "@/lib/workspace-resource-policies";
import { useTemplateType } from "@/hooks/use-template-types";
import {
  useTemplateVersions,
  useCreateTemplateVersion,
  useCloneVersion,
  usePublishVersion,
  useDeleteVersion,
} from "@/hooks/use-template-version";
import {
  useTemplatesByType,
  useCreateTemplate,
  useForkTemplate,
} from "@/hooks/use-templates";
import { useApi } from "@/hooks/use-api";
import { useResolvedWorkspacePolicies } from "@/hooks/use-settings";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ResourceStateBadges } from "@/components/shared/resource-state-badges";
import { StatusBadge } from "@/components/shared/status-badge";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import {
  buildDefaultEditorData,
  DEFAULT_MJML,
} from "@/components/templates/template-version-defaults";
import { getVersionPrimaryAction } from "@/components/templates/templates-list-actions";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { TemplateVersion } from "@/types/templates";
import { toast } from "sonner";
import { useState } from "react";

function createEditorId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

export function TemplatesListContent() {
  const t = useTranslations("templatesPage");
  const tCommon = useTranslations("common");
  const locale = useLocale();
  const router = useRouter();
  const scope = useScope();
  const scopedPath = useScopedPath();
  const workspacePolicies = useResolvedWorkspacePolicies(scope);
  const templateCatalogState = getTemplateCatalogState(scope, workspacePolicies.data);
  const params = useParams<{ slug: string }>();
  const slug = params.slug;

  const { data: templateType, isLoading: typeLoading } = useTemplateType(
    scopedPath,
    slug
  );
  const { data: templates } = useTemplatesByType(scopedPath, slug);

  // Use first template for this type (own scope)
  const template = templates?.[0];
  const templateId = template?.id ?? "";
  const templateState = getTemplateManagementState(
    scope,
    template,
    workspacePolicies.data,
  );

  const { data: versions, isLoading: versionsLoading } = useTemplateVersions(
    scopedPath,
    templateId
  );
  const api = useApi();
  const createTemplate = useCreateTemplate(scopedPath);
  const forkTemplate = useForkTemplate(scopedPath, slug);
  const createVersion = useCreateTemplateVersion(scopedPath, templateId);
  const cloneVersion = useCloneVersion(scopedPath, templateId);
  const deleteVersion = useDeleteVersion(scopedPath, templateId);

  const [deleteVersionTarget, setDeleteVersionTarget] = useState<TemplateVersion | null>(null);
  const [publishTarget, setPublishTarget] = useState<TemplateVersion | null>(
    null
  );

  function buildEditPath(tplId: string, versionId: string): string {
    switch (scope.level) {
      case "global":
        return `/global/templates/${slug}/edit?templateId=${tplId}&versionId=${versionId}`;
      default:
        return `/t/${scope.tenantCode}/w/${scope.workspaceCode}/templates/${slug}/edit?templateId=${tplId}&versionId=${versionId}`;
    }
  }

  async function handleCreateVersion() {
    if (!templateType) {
      toast.error(t("templateTypeNotLoaded"));
      return;
    }
    if (template && !templateState.canManageVersions) {
      toast.error(
        templateState.canFork
          ? t("readOnlyDefaultMustFork")
          : t("versionManagementDisabledInWorkspace"),
      );
      return;
    }
    if (!template && !templateCatalogState.canCreateTemplates) {
      toast.error(t("localTemplateCreationDisabled"));
      return;
    }
    try {
      // Auto-create template if none exists for this type
      let tplId = templateId;
      if (!tplId) {
        const newTemplate = await createTemplate.mutateAsync({
          template_type_id: templateType.id,
        });
        tplId = newTemplate.id;
      }

      // Use API directly with the resolved tplId (hook may have stale templateId)
      const versionData = {
        subject: `${templateType.name ?? slug} — New Draft`,
        from_name: "Senda",
        body_mjml: DEFAULT_MJML,
        default_locale: "en",
        editor_data: buildDefaultEditorData(createEditorId),
      };
      const version = tplId === templateId
        ? await createVersion.mutateAsync(versionData)
        : await api
            .post(`${scopedPath}/templates/${tplId}/versions`, { json: versionData })
            .json<TemplateVersion>();

      toast.success(t("draftVersionCreated"));
      router.push(buildEditPath(tplId, version.id));
    } catch {
      toast.error(t("createVersionFailed"));
    }
  }

  async function handleForkTemplate() {
    if (!templateId) {
      toast.error(t("defaultTemplateNotLoaded"));
      return;
    }

    await forkTemplate.mutateAsync(templateId);
  }

  const publishMutation = usePublishVersion(
    scopedPath,
    templateId,
    publishTarget?.id ?? ""
  );

  async function handlePublish() {
    if (!publishTarget) return;
    try {
      await publishMutation.mutateAsync();
      toast.success(t("publishSuccess", { version: publishTarget.version_number }));
      setPublishTarget(null);
    } catch {
      toast.error(t("publishFailed"));
    }
  }

  const sortedVersions = useMemo(
    () =>
      [...(versions ?? [])].sort(
        (a, b) => b.version_number - a.version_number
      ),
    [versions]
  );
  const versionWriteDisabledReason = template?.is_fork
    ? t("forkVersionManagementOnly")
    : templateState.canFork
      ? t("defaultTemplatesReadonly")
      : t("localTemplateChangesDisabled");
  const canCreateDraftVersion = template
    ? templateState.canManageVersions
    : templateCatalogState.canCreateTemplates;

  const columns: ColumnDef<TemplateVersion>[] = [
    {
      accessorKey: "version_number",
      header: t("columns.version"),
      cell: ({ row }) => (
        <button
          type="button"
          onClick={() => router.push(buildEditPath(templateId, row.original.id))}
          className={`font-mono text-sm transition-colors hover:text-primary ${
            row.original.status === "published" ? "font-semibold" : ""
          }`}
          aria-label={t("openVersionAria", { version: row.original.version_number })}
        >
          v{row.original.version_number}
        </button>
      ),
    },
    {
      accessorKey: "status",
      header: tCommon("status"),
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      accessorKey: "created_by",
      header: t("columns.createdBy"),
      cell: ({ row }) => (
        <span className="text-sm">{row.original.created_by ?? "—"}</span>
      ),
    },
    {
      accessorKey: "created_at",
      header: t("columns.date"),
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {new Intl.DateTimeFormat(locale, {
            year: "numeric",
            month: "2-digit",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
          }).format(new Date(row.original.created_at))}
        </span>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => {
        const primaryAction = getVersionPrimaryAction(
          row.original.status,
          templateState.versionPrimaryAction,
        );

        return (
        <div className="flex items-center justify-end gap-1">
          {row.original.status === "draft" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => router.push(buildEditPath(templateId, row.original.id))}>
                  {primaryAction.icon === "pencil" ? (
                    <Pencil className="h-4 w-4" />
                  ) : (
                    <FileText className="h-4 w-4" />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t(`actions.${primaryAction.labelKey}`)}</TooltipContent>
            </Tooltip>
          )}
          {row.original.status === "draft" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    disabled={!templateState.canManageVersions}
                    onClick={() =>
                      templateState.canManageVersions && setPublishTarget(row.original)
                    }
                  >
                  <Send className="h-4 w-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>
                {templateState.canManageVersions ? t("actions.publish") : versionWriteDisabledReason}
              </TooltipContent>
            </Tooltip>
          )}
          <Tooltip>
            <TooltipTrigger asChild>
              <span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  disabled={cloneVersion.isPending || !templateState.canManageVersions}
                  onClick={() =>
                    templateState.canManageVersions &&
                    cloneVersion.mutate(row.original.id, {
                      onSuccess: () =>
                        toast.success(
                          t("cloneSuccess", { version: row.original.version_number })
                        ),
                      onError: () => toast.error(t("cloneFailed")),
                    })
                  }
                >
                  <Copy className="h-4 w-4" />
                </Button>
              </span>
            </TooltipTrigger>
            <TooltipContent>
              {templateState.canManageVersions ? t("actions.clone") : versionWriteDisabledReason}
            </TooltipContent>
          </Tooltip>
          {row.original.status === "draft" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-destructive"
                    disabled={!templateState.canManageVersions}
                    onClick={() =>
                      templateState.canManageVersions && setDeleteVersionTarget(row.original)
                    }
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>
                {templateState.canManageVersions ? tCommon("delete") : versionWriteDisabledReason}
              </TooltipContent>
            </Tooltip>
          )}
          {row.original.status === "published" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => router.push(buildEditPath(templateId, row.original.id))}>
                  <FileText className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t(`actions.${primaryAction.labelKey}`)}</TooltipContent>
            </Tooltip>
          )}
        </div>
      )},
    },
  ];

  const isLoading = typeLoading || versionsLoading;

  return (
    <div className="flex flex-col gap-6">
      {/* Type info header */}
      {templateType && (
        <div className="flex flex-wrap items-center gap-4">
          <h2 className="text-xl font-semibold" style={{ letterSpacing: "-1px" }}>
            {templateType.name}
          </h2>
          <span className="font-mono text-sm text-muted-foreground">
            {templateType.slug}
          </span>
          <ScopeIndicator scope={resolveResourceDisplayScope(templateType)} />
          <ResourceStateBadges badges={templateState.badges} />
        </div>
      )}

      {template && templateState.readOnly ? (
        <div className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
          <Lock className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="space-y-1">
            <p className="font-medium">
              {t("readOnlyBannerTitle")}
            </p>
            <p>
              {templateState.canFork
                ? t("readOnlyBannerDescription")
                : versionWriteDisabledReason}
            </p>
          </div>
        </div>
      ) : null}

      {/* Versions section */}
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold">{t("versionsTitle")}</h3>
        <div className="flex items-center gap-2">
          {templateState.canFork ? (
            <Button
              type="button"
              variant="outline"
              onClick={handleForkTemplate}
              disabled={forkTemplate.isPending}
            >
              <GitFork className="mr-2 h-4 w-4" />
              {forkTemplate.isPending ? t("forking") : t("forkFromDefault")}
            </Button>
          ) : null}
          <Tooltip>
            <TooltipTrigger asChild>
              <span>
                <Button onClick={handleCreateVersion} disabled={!canCreateDraftVersion}>
                  <Plus className="h-4 w-4 mr-2" />
                  {t("newVersion")}
                </Button>
              </span>
            </TooltipTrigger>
            <TooltipContent>
              {canCreateDraftVersion
                ? t("createDraftVersion")
                : templateState.canFork
                  ? t("forkFirstBeforeVersions")
                  : t("versionManagementDisabledForScope")}
            </TooltipContent>
          </Tooltip>
        </div>
      </div>

      <DataTable
        columns={columns}
        data={sortedVersions}
        loading={isLoading}
        emptyState={
          <EmptyState
            icon={FileText}
            title={t("empty.title")}
            description={t("empty.description")}
            action={
              <div className="flex items-center gap-2">
                {templateState.canFork ? (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleForkTemplate}
                    disabled={forkTemplate.isPending}
                  >
                    <GitFork className="mr-2 h-4 w-4" />
                    {t("forkFromDefault")}
                  </Button>
                ) : null}
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleCreateVersion}
                  disabled={!canCreateDraftVersion}
                >
                  <Plus className="h-4 w-4 mr-2" />
                  {t("createVersion")}
                </Button>
              </div>
            }
          />
        }
      />

      <ConfirmDialog
        open={!!publishTarget}
        onOpenChange={(open) => !open && setPublishTarget(null)}
        title={t("publishDialog.title")}
        description={t("publishDialog.description", { version: publishTarget?.version_number ?? "" })}
        confirmLabel={t("actions.publish")}
        variant="default"
        onConfirm={handlePublish}
        loading={publishMutation.isPending}
      />

      <ConfirmDialog
        open={!!deleteVersionTarget}
        onOpenChange={(open) => !open && setDeleteVersionTarget(null)}
        title={t("deleteDialog.title")}
        description={t("deleteDialog.description", { version: deleteVersionTarget?.version_number ?? "" })}
        confirmLabel={tCommon("delete")}
        onConfirm={() => {
          if (deleteVersionTarget) {
            deleteVersion.mutate(deleteVersionTarget.id, {
              onSuccess: () => toast.success(t("deleteSuccess")),
              onError: () => toast.error(t("deleteFailed")),
            });
            setDeleteVersionTarget(null);
          }
        }}
        loading={deleteVersion.isPending}
      />
    </div>
  );
}

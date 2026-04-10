"use client";

import { useMemo } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { useRouter, useParams } from "next/navigation";
import { Plus, FileText, Pencil, Send, Trash2, Copy } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import { useTemplateType } from "@/hooks/use-template-types";
import {
  useTemplateVersions,
  useCreateTemplateVersion,
  useCloneVersion,
  usePublishVersion,
  useDeleteVersion,
} from "@/hooks/use-template-version";
import { useTemplatesByType, useCreateTemplate } from "@/hooks/use-templates";
import { useApi } from "@/hooks/use-api";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
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
  const router = useRouter();
  const scope = useScope();
  const scopedPath = useScopedPath();
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

  const { data: versions, isLoading: versionsLoading } = useTemplateVersions(
    scopedPath,
    templateId
  );
  const api = useApi();
  const createTemplate = useCreateTemplate(scopedPath);
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
      toast.error("Template type not loaded");
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

      toast.success("Draft version created");
      router.push(buildEditPath(tplId, version.id));
    } catch {
      toast.error("Failed to create version");
    }
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
      toast.success(`Version ${publishTarget.version_number} published`);
      setPublishTarget(null);
    } catch {
      toast.error("Failed to publish version");
    }
  }

  const sortedVersions = useMemo(
    () =>
      [...(versions ?? [])].sort(
        (a, b) => b.version_number - a.version_number
      ),
    [versions]
  );

  const columns: ColumnDef<TemplateVersion>[] = [
    {
      accessorKey: "version_number",
      header: "VERSION",
      cell: ({ row }) => (
        <button
          type="button"
          onClick={() => router.push(buildEditPath(templateId, row.original.id))}
          className={`font-mono text-sm transition-colors hover:text-primary ${
            row.original.status === "published" ? "font-semibold" : ""
          }`}
          aria-label={`Open version ${row.original.version_number}`}
        >
          v{row.original.version_number}
        </button>
      ),
    },
    {
      accessorKey: "status",
      header: "STATUS",
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      accessorKey: "created_by",
      header: "CREATED BY",
      cell: ({ row }) => (
        <span className="text-sm">{row.original.created_by ?? "—"}</span>
      ),
    },
    {
      accessorKey: "created_at",
      header: "DATE",
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {new Date(row.original.created_at).toLocaleString("en-US", {
            year: "numeric",
            month: "2-digit",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </span>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => {
        const primaryAction = getVersionPrimaryAction(row.original.status);

        return (
        <div className="flex items-center justify-end gap-1">
          {row.original.status === "draft" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => router.push(buildEditPath(templateId, row.original.id))}>
                  <Pencil className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{primaryAction.label}</TooltipContent>
            </Tooltip>
          )}
          {row.original.status === "draft" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setPublishTarget(row.original)}>
                  <Send className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Publish</TooltipContent>
            </Tooltip>
          )}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                disabled={cloneVersion.isPending}
                onClick={() =>
                  cloneVersion.mutate(row.original.id, {
                    onSuccess: () =>
                      toast.success(
                        `Version ${row.original.version_number} cloned as draft`
                      ),
                    onError: () => toast.error("Failed to clone version"),
                  })
                }
              >
                <Copy className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Clone</TooltipContent>
          </Tooltip>
          {row.original.status === "draft" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => setDeleteVersionTarget(row.original)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Delete</TooltipContent>
            </Tooltip>
          )}
          {row.original.status === "published" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => router.push(buildEditPath(templateId, row.original.id))}>
                  <FileText className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{primaryAction.label}</TooltipContent>
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
        <div className="flex items-center gap-4">
          <h2
            className="text-xl font-semibold"
            style={{ letterSpacing: "-1px" }}
          >
            {templateType.name}
          </h2>
          <span className="font-mono text-sm text-muted-foreground">
            {templateType.slug}
          </span>
          <ScopeIndicator scope={templateType.scope_level} />
        </div>
      )}

      {/* Versions section */}
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold">Versions</h3>
        <Button onClick={handleCreateVersion} disabled={!templateId}>
          <Plus className="h-4 w-4 mr-2" />
          New Version
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={sortedVersions}
        loading={isLoading}
        emptyState={
          <EmptyState
            icon={FileText}
            title="No versions yet"
            description="Create your first template version to start building this email template."
            action={
              <Button
                variant="outline"
                size="sm"
                onClick={handleCreateVersion}
              >
                <Plus className="h-4 w-4 mr-2" />
                Create Version
              </Button>
            }
          />
        }
      />

      <ConfirmDialog
        open={!!publishTarget}
        onOpenChange={(open) => !open && setPublishTarget(null)}
        title="Publish Version"
        description={`Are you sure you want to publish version ${publishTarget?.version_number}? This will replace the current published version.`}
        confirmLabel="Publish"
        variant="default"
        onConfirm={handlePublish}
        loading={publishMutation.isPending}
      />

      <ConfirmDialog
        open={!!deleteVersionTarget}
        onOpenChange={(open) => !open && setDeleteVersionTarget(null)}
        title="Delete Draft Version"
        description={`Are you sure you want to delete version ${deleteVersionTarget?.version_number}? This action cannot be undone.`}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteVersionTarget) {
            deleteVersion.mutate(deleteVersionTarget.id, {
              onSuccess: () => toast.success("Draft version deleted"),
              onError: () => toast.error("Failed to delete version"),
            });
            setDeleteVersionTarget(null);
          }
        }}
        loading={deleteVersion.isPending}
      />
    </div>
  );
}

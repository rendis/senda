"use client";

import { useState, useEffect } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import {
  Plus,
  Webhook as WebhookIcon,
  Pencil,
  Trash2,
  Zap,
  Power,
} from "lucide-react";
import { toast } from "sonner";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { WebhookForm } from "./webhook-form";
import {
  useWebhooks,
  useCreateWebhook,
  useUpdateWebhook,
  useDeleteWebhook,
  useTestWebhook,
} from "@/hooks/use-webhooks";
import { useScope } from "@/hooks/use-scope";
import { truncate } from "@/lib/utils";
import type { Webhook, WebhookEventType } from "@/types/webhooks";

const EVENT_COLORS: Record<string, { dot: string; bg: string; text: string }> = {
  "email.sent": { dot: "bg-status-sent", bg: "bg-status-sent-bg", text: "text-status-sent" },
  "email.delivered": { dot: "bg-status-delivered", bg: "bg-status-delivered-bg", text: "text-status-delivered" },
  "email.bounced": { dot: "bg-status-bounced", bg: "bg-status-bounced-bg", text: "text-status-bounced" },
  "email.complained": { dot: "bg-status-complained", bg: "bg-status-complained-bg", text: "text-status-complained" },
  "email.opened": { dot: "bg-scope-system", bg: "bg-scope-system-bg", text: "text-scope-system" },
};

function EventBadge({ event }: { event: WebhookEventType }) {
  const label = event.replace("email.", "");
  const capitalLabel = label.charAt(0).toUpperCase() + label.slice(1);
  const colors = EVENT_COLORS[event] ?? {
    dot: "bg-status-draft",
    bg: "bg-status-draft-bg",
    text: "text-status-draft",
  };

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 h-6 font-mono text-[11px] font-medium ${colors.bg} ${colors.text}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${colors.dot}`} />
      {capitalLabel}
    </span>
  );
}

export function WebhooksContent() {
  const scope = useScope();

  if (scope.level !== "workspace") {
    return (
      <EmptyState
        icon={WebhookIcon}
        title="Select a workspace"
        description="Webhooks are workspace-scoped. Select a workspace from the sidebar to manage webhooks."
      />
    );
  }

  return <WebhooksTable />;
}

function WebhooksTable() {
  const { data, isLoading, error, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useWebhooks();
  const createMutation = useCreateWebhook();
  const updateMutation = useUpdateWebhook();
  const deleteMutation = useDeleteWebhook();
  const testMutation = useTestWebhook();

  const [formOpen, setFormOpen] = useState(false);
  const [editingWebhook, setEditingWebhook] = useState<Webhook | undefined>();
  const [createdSecret, setCreatedSecret] = useState<string | undefined>();
  const [deleteTarget, setDeleteTarget] = useState<Webhook | null>(null);

  useEffect(() => {
    if (error) toast.error("Failed to load webhooks");
  }, [error]);

  const webhooks = data?.pages.flatMap((p) => p.items) ?? [];

  const handleCreate = async (formData: {
    url: string;
    events: WebhookEventType[];
  }) => {
    const result = await createMutation.mutateAsync(formData);
    if (result.secret) {
      setCreatedSecret(result.secret);
    } else {
      setFormOpen(false);
      toast.success("Webhook created");
    }
  };

  const handleUpdate = async (formData: {
    url: string;
    events: WebhookEventType[];
  }) => {
    if (!editingWebhook) return;
    await updateMutation.mutateAsync({
      id: editingWebhook.id,
      data: formData,
    });
    setEditingWebhook(undefined);
    setFormOpen(false);
    toast.success("Webhook updated");
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    await deleteMutation.mutateAsync(deleteTarget.id);
    setDeleteTarget(null);
    toast.success("Webhook deleted");
  };

  const handleTest = async (webhook: Webhook) => {
    try {
      const result = await testMutation.mutateAsync(webhook.id);
      if (result.status_code) {
        toast.success(
          `Test webhook: ${result.status_code} (${result.latency_ms ?? 0}ms)`
        );
      } else {
        toast.success("Test webhook sent");
      }
    } catch {
      toast.error("Test webhook failed");
    }
  };

  const handleToggleActive = async (webhook: Webhook) => {
    await updateMutation.mutateAsync({
      id: webhook.id,
      data: { is_active: !webhook.is_active },
    });
    toast.success(
      webhook.is_active ? "Webhook disabled" : "Webhook enabled"
    );
  };

  const openEdit = (webhook: Webhook) => {
    setEditingWebhook(webhook);
    setCreatedSecret(undefined);
    setFormOpen(true);
  };

  const openCreate = () => {
    setEditingWebhook(undefined);
    setCreatedSecret(undefined);
    setFormOpen(true);
  };

  const columns: ColumnDef<Webhook>[] = [
    {
      accessorKey: "url",
      header: "URL",
      cell: ({ row }) => (
        <span className="font-mono text-xs text-foreground">
          {truncate(row.original.url, 50)}
        </span>
      ),
    },
    {
      accessorKey: "events",
      header: "Events",
      size: 280,
      enableSorting: false,
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1">
          {(row.original.events ?? []).map((evt) => (
            <EventBadge key={evt} event={evt} />
          ))}
        </div>
      ),
    },
    {
      accessorKey: "is_active",
      header: "Status",
      size: 80,
      cell: ({ row }) => (
        <span
          className={`text-xs font-medium ${
            row.original.is_active ? "text-status-delivered" : "text-muted-foreground"
          }`}
        >
          {row.original.is_active ? "Active" : "Disabled"}
        </span>
      ),
    },
    {
      accessorKey: "consecutive_failures",
      header: "Failures",
      size: 80,
      cell: ({ row }) => {
        const failures = row.original.consecutive_failures;
        return (
          <span
            className={`font-mono text-xs ${
              failures > 0
                ? "font-semibold text-status-complained"
                : "text-muted-foreground"
            }`}
          >
            {failures}
          </span>
        );
      },
    },
    {
      id: "actions",
      size: 40,
      enableSorting: false,
      cell: ({ row }) => {
        const webhook = row.original;
        return (
          <div className="flex items-center justify-end gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => handleTest(webhook)}>
                  <Zap className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Test Webhook</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => openEdit(webhook)}>
                  <Pencil className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Edit</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => handleToggleActive(webhook)}>
                  <Power className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{webhook.is_active ? "Disable" : "Enable"}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => setDeleteTarget(webhook)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Delete</TooltipContent>
            </Tooltip>
          </div>
        );
      },
    },
  ];

  return (
    <>
      <div className="flex items-center mb-6">
        <Button onClick={openCreate}>
          <Plus className="h-4 w-4 mr-2" />
          Nuevo Webhook
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={webhooks}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={WebhookIcon}
            title="No webhooks configured"
            description="Create a webhook to receive real-time notifications when email events occur."
            action={
              <Button onClick={openCreate}>
                <Plus className="h-4 w-4 mr-2" />
                Nuevo Webhook
              </Button>
            }
          />
        }
      />

      <WebhookForm
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open);
          if (!open) {
            setEditingWebhook(undefined);
            setCreatedSecret(undefined);
          }
        }}
        webhook={editingWebhook}
        createdSecret={createdSecret}
        onSubmit={editingWebhook ? handleUpdate : handleCreate}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete Webhook"
        description={`Are you sure you want to delete the webhook for ${deleteTarget?.url ?? ""}? This action cannot be undone.`}
        confirmLabel="Delete"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </>
  );
}

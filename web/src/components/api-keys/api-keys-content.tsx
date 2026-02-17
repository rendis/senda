"use client";

import { useState, useEffect } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { KeyRound, MoreHorizontal, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { StatusBadge } from "@/components/shared/status-badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ApiKeyGenerateDialog } from "./api-key-generate-dialog";
import {
  useApiKeys,
  useCreateApiKey,
  useRevokeApiKey,
} from "@/hooks/use-api-keys-mgmt";
import { formatDate, formatRelativeTime } from "@/lib/utils";
import { useScope } from "@/hooks/use-scope";
import type { ApiKey } from "@/types/api-keys";

export function ApiKeysContent() {
  const { level } = useScope();

  if (level !== "workspace") {
    return (
      <EmptyState
        icon={KeyRound}
        title="Select a workspace"
        description="API keys are workspace-scoped. Switch to a workspace to manage API keys."
      />
    );
  }

  return <ApiKeysTable />;
}

function ApiKeysTable() {
  const { data, isLoading, error, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useApiKeys();
  const createMutation = useCreateApiKey();
  const revokeMutation = useRevokeApiKey();

  const [generateOpen, setGenerateOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<ApiKey | null>(null);

  useEffect(() => {
    if (error) toast.error("Failed to load API keys");
  }, [error]);

  const apiKeys = data?.pages.flatMap((p) => p.items) ?? [];

  const handleGenerate = async (name: string) => {
    const result = await createMutation.mutateAsync({ name });
    toast.success("API key generated");
    return result;
  };

  const handleRevoke = async () => {
    if (!revokeTarget) return;
    await revokeMutation.mutateAsync(revokeTarget.id);
    setRevokeTarget(null);
    toast.success("API key revoked");
  };

  const columns: ColumnDef<ApiKey>[] = [
    {
      accessorKey: "name",
      header: "Name",
      cell: ({ row }) => (
        <span className="text-sm font-medium">{row.original.name}</span>
      ),
    },
    {
      accessorKey: "masked_key",
      header: "Hint",
      size: 140,
      enableSorting: false,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {row.original.masked_key}
        </span>
      ),
    },
    {
      id: "created_at",
      header: "Date",
      size: 140,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {formatDate(row.original.created_at).split(",")[0]}
        </span>
      ),
    },
    {
      id: "last_used",
      header: "Last Used",
      size: 140,
      cell: ({ row }) => (
        <span className="text-xs text-muted-foreground">
          {row.original.last_used_at
            ? formatRelativeTime(row.original.last_used_at)
            : "Never"}
        </span>
      ),
    },
    {
      id: "status",
      header: "Status",
      size: 80,
      cell: () => <StatusBadge status="active" />,
    },
    {
      id: "actions",
      size: 40,
      enableSorting: false,
      cell: ({ row }) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="p-1 rounded hover:bg-muted">
              <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setRevokeTarget(row.original)}
            >
              <Trash2 className="h-4 w-4" />
              Revoke
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <>
      <div className="flex items-center mb-6">
        <Button onClick={() => setGenerateOpen(true)}>
          <KeyRound className="h-4 w-4 mr-2" />
          Generar API Key
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={apiKeys}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={KeyRound}
            title="No API keys"
            description="Generate an API key to access the Senda API programmatically."
            action={
              <Button onClick={() => setGenerateOpen(true)}>
                <KeyRound className="h-4 w-4 mr-2" />
                Generar API Key
              </Button>
            }
          />
        }
      />

      <ApiKeyGenerateDialog
        open={generateOpen}
        onOpenChange={setGenerateOpen}
        onGenerate={handleGenerate}
      />

      <ConfirmDialog
        open={!!revokeTarget}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null);
        }}
        title="Revoke API Key"
        description={`Are you sure you want to revoke "${revokeTarget?.name ?? ""}"? Applications using this key will lose access immediately.`}
        confirmLabel="Revoke"
        onConfirm={handleRevoke}
        loading={revokeMutation.isPending}
      />
    </>
  );
}

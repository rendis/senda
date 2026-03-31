"use client";

import { useState } from "react";
import { Plug, Plus, Check, Trash2, Zap, Pencil, Mail } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useAdapterList,
  useCreateAdapter,
  useDeleteAdapter,
  useUpdateAdapter,
  useTestAdapterSend,
  useAutoProvision,
} from "@/hooks/use-adapters";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { AdapterTypeBadge } from "./adapter-type-badge";
import { AdapterForm } from "./adapter-form";
import { ProvisioningStepper } from "./provisioning-stepper";
import { IdentityPanel } from "./identity-panel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ColumnDef } from "@tanstack/react-table";
import type { Adapter, CreateAdapterRequest, ProvisioningOverallStatus } from "@/types/adapters";
import { TrackingStatus } from "./tracking-status";
import { DefaultSender } from "./default-sender";

export function AdaptersContent() {
  const { level } = useScope();

  if (level === "tenant") {
    return (
      <EmptyState
        icon={Plug}
        title="Select a workspace"
        description="Adapters are workspace-scoped. Select a workspace from the sidebar to manage adapters."
      />
    );
  }

  return <AdaptersTable />;
}

function AdaptersTable() {
  const scopedPath = useScopedPath();
  const [search, setSearch] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<Adapter | null>(null);
  const [editTarget, setEditTarget] = useState<Adapter | null>(null);
  const [testTarget, setTestTarget] = useState<Adapter | null>(null);
  const [provisionTarget, setProvisionTarget] = useState<Adapter | null>(null);
  const [identityTarget, setIdentityTarget] = useState<Adapter | null>(null);

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
    const adapter = await createAdapter.mutateAsync(data);
    // Auto-trigger provisioning for SES adapters
    if (data.adapter_type === "ses" && adapter?.id) {
      setProvisionTarget(adapter);
    }
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
      id: "sender",
      header: "SENDER",
      cell: ({ row }) => (
        <DefaultSender adapter={row.original} scopedPath={scopedPath} />
      ),
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
      id: "tracking",
      header: "TRACKING",
      cell: ({ row }) =>
        row.original.adapter_type === "ses" ? (
          <TrackingStatus
            adapterId={row.original.id}
            scopedPath={scopedPath}
            onClick={() => setProvisionTarget(row.original)}
          />
        ) : (
          <div className="flex items-center gap-1.5">
            <div className="h-2 w-2 rounded-full bg-muted-foreground/25" />
            <span className="text-[10px] text-muted-foreground/50">Not supported</span>
          </div>
        ),
    },
    {
      id: "actions",
      enableSorting: false,
      cell: ({ row }) => (
        <AdapterActions
          adapter={row.original}
          onDelete={setDeleteTarget}
          onEdit={setEditTarget}
          onTest={setTestTarget}
          onIdentities={setIdentityTarget}
        />
      ),
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

      {/* Edit dialog */}
      {editTarget && (
        <EditAdapterDialog
          key={editTarget.id}
          adapter={editTarget}
          open={!!editTarget}
          onOpenChange={(open) => !open && setEditTarget(null)}
        />
      )}

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

      {/* Test send dialog */}
      {testTarget && (
        <TestSendDialog
          adapter={testTarget}
          open={!!testTarget}
          onOpenChange={(open) => !open && setTestTarget(null)}
        />
      )}

      {/* Provisioning stepper dialog */}
      {provisionTarget && (
        <ProvisioningStepper
          adapter={provisionTarget}
          open={!!provisionTarget}
          onOpenChange={(open) => !open && setProvisionTarget(null)}
        />
      )}

      {/* Identity panel dialog */}
      {identityTarget && (
        <IdentityPanel
          adapter={identityTarget}
          open={!!identityTarget}
          onOpenChange={(open) => !open && setIdentityTarget(null)}
        />
      )}
    </div>
  );
}

function AdapterActions({
  adapter,
  onDelete,
  onEdit,
  onTest,
  onIdentities,
}: {
  adapter: Adapter;
  onDelete: (a: Adapter) => void;
  onEdit: (a: Adapter) => void;
  onTest: (a: Adapter) => void;
  onIdentities: (a: Adapter) => void;
}) {
  return (
    <div className="flex items-center justify-end gap-1">
      {adapter.adapter_type === "ses" && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onIdentities(adapter)}>
              <Mail className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Senders</TooltipContent>
        </Tooltip>
      )}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onEdit(adapter)}>
            <Pencil className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>Edit</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onTest(adapter)}>
            <Zap className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>Test Send</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => onDelete(adapter)}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>Delete</TooltipContent>
      </Tooltip>
    </div>
  );
}

function EditAdapterDialog({
  adapter,
  open,
  onOpenChange,
}: {
  adapter: Adapter;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scopedPath = useScopedPath();
  const updateAdapter = useUpdateAdapter(scopedPath, adapter.id);

  return (
    <AdapterForm
      mode="edit"
      adapter={adapter}
      trigger={<span />}
      open={open}
      onOpenChange={onOpenChange}
      onSubmit={async (data) => {
        await updateAdapter.mutateAsync(data);
      }}
    />
  );
}

function TestSendDialog({
  adapter,
  open,
  onOpenChange,
}: {
  adapter: Adapter;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scopedPath = useScopedPath();
  const testSend = useTestAdapterSend(scopedPath, adapter.id);
  const [to, setTo] = useState("");
  const [subject, setSubject] = useState("Test email from Senda");
  const [body, setBody] = useState(
    "<h1>Test Email</h1><p>This is a test email sent from Senda to verify the adapter configuration.</p>"
  );

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    testSend.mutate(
      { to, subject, body },
      { onSuccess: () => onOpenChange(false) }
    );
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !testSend.isPending && onOpenChange(v)}>
      <DialogContent onInteractOutside={(e) => testSend.isPending && e.preventDefault()}>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Test Send — {adapter.name}</DialogTitle>
            <DialogDescription>
              Send a test email through this adapter to verify it works.
            </DialogDescription>
          </DialogHeader>
          <fieldset disabled={testSend.isPending} className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="test-to">Recipient</Label>
              <Input
                id="test-to"
                type="email"
                value={to}
                onChange={(e) => setTo(e.target.value)}
                placeholder="recipient@example.com"
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="test-subject">Subject</Label>
              <Input
                id="test-subject"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                placeholder="Test email"
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="test-body">Body (HTML)</Label>
              <textarea
                id="test-body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                rows={4}
                required
                className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono"
              />
            </div>
          </fieldset>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={testSend.isPending}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={testSend.isPending}>
              {testSend.isPending ? "Sending..." : "Send Test"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

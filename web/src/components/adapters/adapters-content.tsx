"use client";

import { useState } from "react";
import { Plug, Plus, MoreHorizontal, Check, Trash2, Zap, Pencil } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useAdapterList,
  useCreateAdapter,
  useDeleteAdapter,
  useUpdateAdapter,
  useTestAdapterSend,
} from "@/hooks/use-adapters";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { AdapterTypeBadge } from "./adapter-type-badge";
import { AdapterForm } from "./adapter-form";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ColumnDef } from "@tanstack/react-table";
import type { Adapter, CreateAdapterRequest } from "@/types/adapters";

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
      id: "sender",
      header: "SENDER",
      cell: ({ row }) => {
        const meta = row.original.config_meta;
        const value = meta?.delegate_email || meta?.region || "\u2014";
        return (
          <span className="font-mono text-[13px] text-muted-foreground">
            {value}
          </span>
        );
      },
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
      cell: ({ row }) => (
        <AdapterActions
          adapter={row.original}
          onDelete={setDeleteTarget}
          onEdit={setEditTarget}
          onTest={setTestTarget}
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
    </div>
  );
}

function AdapterActions({
  adapter,
  onDelete,
  onEdit,
  onTest,
}: {
  adapter: Adapter;
  onDelete: (a: Adapter) => void;
  onEdit: (a: Adapter) => void;
  onTest: (a: Adapter) => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
          <MoreHorizontal className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => onEdit(adapter)}>
          <Pencil className="mr-2 h-4 w-4" />
          Edit
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onTest(adapter)}>
          <Zap className="mr-2 h-4 w-4" />
          Test Send
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
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Test Send — {adapter.name}</DialogTitle>
            <DialogDescription>
              Send a test email through this adapter to verify it works.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
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
          </div>
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

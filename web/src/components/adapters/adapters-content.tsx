"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Plug, Plus, Trash2, Zap, Pencil, Mail, Share2, Lock } from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import {
  useAdapterList,
  useAdapterWorkspaceAccess,
  useCreateAdapter,
  useDeleteAdapter,
  useUpdateAdapter,
  useUpdateAdapterWorkspaceAccess,
  useTestAdapterSend,
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
import { useWorkspacesManagement } from "@/hooks/use-workspaces-mgmt";
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
import type { Adapter, AdapterIdentity, CreateAdapterRequest } from "@/types/adapters";
import { useIdentityList } from "@/hooks/use-identities";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { TrackingStatus } from "./tracking-status";
import { DefaultSender } from "./default-sender";

const isVerifiedEmail = (i: AdapterIdentity) =>
  i.identity_type === "email" && i.status === "verified";

export function AdaptersContent() {
  return <AdaptersTable />;
}

function AdaptersTable() {
  const scope = useScope();
  const scopedPath = useScopedPath();
  const [search, setSearch] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<Adapter | null>(null);
  const [editTarget, setEditTarget] = useState<Adapter | null>(null);
  const [testTarget, setTestTarget] = useState<Adapter | null>(null);
  const [provisionTarget, setProvisionTarget] = useState<Adapter | null>(null);
  const [identityTarget, setIdentityTarget] = useState<Adapter | null>(null);
  const [shareTarget, setShareTarget] = useState<Adapter | null>(null);

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
        <div className="flex items-center gap-2">
          <span className="font-medium text-[13px] text-foreground">
            {row.original.name}
          </span>
          {row.original.is_shared && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="inline-flex items-center gap-1 rounded-full bg-scope-system-bg px-2 py-0.5 text-[10px] font-medium text-scope-system">
                  <Lock className="h-3 w-3" />
                  Shared
                </span>
              </TooltipTrigger>
              <TooltipContent>Shared from _system — read only in this workspace</TooltipContent>
            </Tooltip>
          )}
        </div>
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
      cell: ({ row }) => (
        <ScopeIndicator
          scope={
            row.original.source_scope === "system" ||
            scope.workspaceCode === SYSTEM_WORKSPACE_CODE
              ? "system"
              : "workspace"
          }
        />
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
            isShared={row.original.is_shared}
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
          scopedPath={scopedPath}
          isSystemWorkspace={scope.workspaceCode === SYSTEM_WORKSPACE_CODE}
          onDelete={setDeleteTarget}
          onEdit={setEditTarget}
          onTest={setTestTarget}
          onIdentities={setIdentityTarget}
          onShare={setShareTarget}
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
          scopedPath={scopedPath}
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

      {shareTarget && (
        <AdapterWorkspaceAccessDialog
          adapter={shareTarget}
          open={!!shareTarget}
          onOpenChange={(open) => !open && setShareTarget(null)}
        />
      )}
    </div>
  );
}

function AdapterActions({
  adapter,
  scopedPath,
  isSystemWorkspace,
  onDelete,
  onEdit,
  onTest,
  onIdentities,
  onShare,
}: {
  adapter: Adapter;
  scopedPath: string;
  isSystemWorkspace: boolean;
  onDelete: (a: Adapter) => void;
  onEdit: (a: Adapter) => void;
  onTest: (a: Adapter) => void;
  onIdentities: (a: Adapter) => void;
  onShare: (a: Adapter) => void;
}) {
  const { data: identities } = useIdentityList(
    scopedPath,
    adapter.adapter_type === "ses" ? adapter.id : "",
  );

  const hasVerifiedSender =
    adapter.adapter_type !== "ses" ||
    (identities ?? []).some(isVerifiedEmail);

  const readOnlyReason = adapter.is_shared
    ? "Shared from _system — read only in this workspace"
    : "This adapter is read only";

  const actionButton = (
    label: string,
    icon: ReactNode,
    onClick: () => void,
    disabled = false,
    destructive = false,
    disabledReason?: string,
  ) => (
    <Tooltip>
      <TooltipTrigger asChild>
        <span>
          <Button
            variant="ghost"
            size="icon"
            className={`h-8 w-8 ${destructive ? "text-destructive" : ""}`}
            onClick={onClick}
            disabled={disabled}
          >
            {icon}
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent>{disabled ? (disabledReason ?? readOnlyReason) : label}</TooltipContent>
    </Tooltip>
  );

  return (
    <div className="flex items-center justify-end gap-1">
      {adapter.adapter_type === "ses" && (
        actionButton("Senders", <Mail className="h-4 w-4" />, () => onIdentities(adapter))
      )}
      {isSystemWorkspace && adapter.adapter_type === "gmail" &&
        actionButton("Workspace access", <Share2 className="h-4 w-4" />, () => onShare(adapter))}
      {actionButton("Edit", <Pencil className="h-4 w-4" />, () => onEdit(adapter), !adapter.is_editable)}
      {actionButton(
        "Test Send",
        <Zap className="h-4 w-4" />,
        () => onTest(adapter),
        !hasVerifiedSender,
        false,
        "No verified sender emails — add and verify an email identity first",
      )}
      {actionButton("Delete", <Trash2 className="h-4 w-4" />, () => onDelete(adapter), !adapter.is_editable, true)}
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

function AdapterWorkspaceAccessDialog({
  adapter,
  open,
  onOpenChange,
}: {
  adapter: Adapter;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scope = useScope();
  const scopedPath = useScopedPath();
  const tenantCode = scope.tenantCode ?? "";
  const { data: access, isLoading } = useAdapterWorkspaceAccess(scopedPath, adapter.id);
  const updateAccess = useUpdateAdapterWorkspaceAccess(scopedPath, adapter.id);
  const { data: workspacePages } = useWorkspacesManagement(tenantCode, "");

  const allWorkspaces = useMemo(
    () =>
      workspacePages?.pages
        .flatMap((page) => page.items)
        .filter((workspace) => !workspace.is_system) ?? [],
    [workspacePages]
  );
  const [selected, setSelected] = useState<string[] | null>(null);

  const effectiveSelection = selected
    ? selected
    : access?.items.filter((item) => item.is_granted).map((item) => item.workspace_id) ?? [];
  const items = access?.items.length
    ? access.items
    : allWorkspaces.map((workspace) => ({
        workspace_id: workspace.id,
        code: workspace.code,
        name: workspace.name,
        is_granted: false,
      }));

  function toggle(workspaceId: string) {
    setSelected((current) =>
      (current ?? effectiveSelection).includes(workspaceId)
        ? (current ?? effectiveSelection).filter((id) => id !== workspaceId)
        : [...(current ?? effectiveSelection), workspaceId]
    );
  }

  async function handleSave() {
    await updateAccess.mutateAsync(effectiveSelection);
    setSelected(null);
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !updateAccess.isPending && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Workspace access — {adapter.name}</DialogTitle>
          <DialogDescription>
            Choose which workspaces can use this Gmail adapter from _system.
          </DialogDescription>
        </DialogHeader>
        <div className="flex max-h-[50vh] flex-col gap-3 overflow-y-auto py-2">
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading workspaces...</p>
          ) : items.length === 0 ? (
            <p className="text-sm text-muted-foreground">No child workspaces available.</p>
          ) : (
            items.map((item) => {
              const checked = effectiveSelection.includes(item.workspace_id);
              return (
                <label key={item.workspace_id} className="flex items-center justify-between rounded-md border px-3 py-2">
                  <div className="flex flex-col">
                    <span className="text-sm font-medium">{item.name}</span>
                    <span className="text-xs text-muted-foreground">{item.code}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggle(item.workspace_id)}
                    className="h-4 w-4"
                  />
                </label>
              );
            })
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={updateAccess.isPending}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={updateAccess.isPending}>
            Save access
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function TestSendDialog({
  adapter,
  scopedPath,
  open,
  onOpenChange,
}: {
  adapter: Adapter;
  scopedPath: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const testSend = useTestAdapterSend(scopedPath, adapter.id);
  const [to, setTo] = useState("");
  const [selectedFrom, setSelectedFrom] = useState<string | undefined>();
  const [subject, setSubject] = useState("Test email from Senda");
  const [body, setBody] = useState(
    "<h1>Test Email</h1><p>This is a test email sent from Senda to verify the adapter configuration.</p>"
  );

  const { data: identities } = useIdentityList(scopedPath, adapter.id);
  const verifiedEmails = useMemo(
    () => (identities ?? []).filter(isVerifiedEmail),
    [identities],
  );

  const defaultFrom =
    verifiedEmails.find((i) => i.is_default)?.identity ??
    verifiedEmails[0]?.identity ??
    "";
  const from = selectedFrom ?? defaultFrom;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    testSend.mutate(
      {
        to,
        subject,
        body,
        ...(adapter.adapter_type === "ses" && from ? { from } : {}),
      },
      { onSuccess: () => onOpenChange(false) },
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
            {adapter.adapter_type === "ses" && (
              <div className="flex flex-col gap-2">
                <Label htmlFor="test-from">Send From</Label>
                <Select value={from} onValueChange={setSelectedFrom} required>
                  <SelectTrigger id="test-from" className="w-full">
                    <SelectValue placeholder="Select sender email..." />
                  </SelectTrigger>
                  <SelectContent>
                    {verifiedEmails.map((identity) => (
                      <SelectItem key={identity.id} value={identity.identity}>
                        {identity.display_name
                          ? `${identity.display_name} <${identity.identity}>`
                          : identity.identity}
                        {identity.is_default ? " (default)" : ""}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
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

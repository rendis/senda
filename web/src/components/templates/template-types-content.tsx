"use client";

import { useState, useMemo } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { useRouter } from "next/navigation";
import {
  Plus,
  Search,
  FileType,
  TriangleAlert,
  Pencil,
  Eye,
  Trash2,
} from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import { generateSlug } from "@/lib/validations/slug";
import { useTemplateTypes, useCreateTemplateType, useUpdateTemplateType, useDeleteTemplateType } from "@/hooks/use-template-types";
import { useAdapterList } from "@/hooks/use-adapters";
import { useIdentityList } from "@/hooks/use-identities";
import { DataTable } from "@/components/shared/data-table";

const SENDER_DEFAULT = "__default__";
const SENDER_NONE = "__none__";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { FormDialog } from "@/components/shared/form-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { TemplateType } from "@/types/templates";
import { toast } from "sonner";

export function TemplateTypesContent() {
  const scope = useScope();

  if (scope.level === "tenant") {
    return (
      <EmptyState
        icon={FileType}
        title="Select a workspace"
        description="Template types are workspace-scoped. Select a workspace from the sidebar to manage template types."
      />
    );
  }

  return <TemplateTypesTable />;
}

function TemplateTypesTable() {
  const router = useRouter();
  const scope = useScope();
  const scopedPath = useScopedPath();
  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useTemplateTypes(scopedPath);
  const createMutation = useCreateTemplateType(scopedPath);
  const deleteMutation = useDeleteTemplateType(scopedPath);
  const { data: adapterData } = useAdapterList(scopedPath);
  const adapters = useMemo(
    () => adapterData?.pages.flatMap((p) => p.items) ?? [],
    [adapterData]
  );

  const [search, setSearch] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<TemplateType | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [newSlug, setNewSlug] = useState("");
  const [newName, setNewName] = useState("");
  const [newAdapterId, setNewAdapterId] = useState("");
  const [newSenderIdentityId, setNewSenderIdentityId] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [editTarget, setEditTarget] = useState<TemplateType | null>(null);

  const allItems = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data]
  );

  const filtered = useMemo(() => {
    if (!search) return allItems;
    const q = search.toLowerCase();
    return allItems.filter(
      (t) =>
        t.slug.toLowerCase().includes(q) ||
        t.name.toLowerCase().includes(q)
    );
  }, [allItems, search]);

  function buildTypePath(slug: string) {
    switch (scope.level) {
      case "global":
        return `/global/templates/${slug}`;
      case "tenant":
        return `/t/${scope.tenantCode}/templates/${slug}`;
      case "workspace":
        return `/t/${scope.tenantCode}/w/${scope.workspaceCode}/templates/${slug}`;
    }
  }

  const columns: ColumnDef<TemplateType>[] = [
    {
      accessorKey: "slug",
      header: "SLUG",
      cell: ({ row }) => (
        <button
          className="font-mono text-sm font-medium text-primary hover:underline"
          onClick={() => router.push(buildTypePath(row.original.slug))}
        >
          {row.original.slug}
        </button>
      ),
    },
    {
      accessorKey: "name",
      header: "NAME",
      cell: ({ row }) => (
        <span className="text-sm text-foreground">{row.original.name}</span>
      ),
    },
    {
      accessorKey: "adapter_id",
      header: "ADAPTER",
      cell: ({ row }) => {
        const aid = row.original.adapter_id;
        if (!aid) {
          return (
            <span className="inline-flex items-center gap-1 text-status-complained text-xs">
              <TriangleAlert className="h-3.5 w-3.5" />
              No adapter
            </span>
          );
        }
        const adapter = adapters.find((a) => a.id === aid);
        return (
          <span className="text-xs text-foreground">
            {adapter?.name ?? aid.slice(0, 8)}
          </span>
        );
      },
    },
    {
      accessorKey: "scope_level",
      header: "SCOPE",
      cell: ({ row }) => (
        <ScopeIndicator scope={row.original.scope_level} />
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex items-center justify-end gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => router.push(buildTypePath(row.original.slug))}>
                <Eye className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>View templates</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setEditTarget(row.original)}>
                <Pencil className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Edit</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => setDeleteTarget(row.original)}>
                <Trash2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Delete</TooltipContent>
          </Tooltip>
        </div>
      ),
    },
  ];

  async function handleCreateType() {
    if (!newSlug.trim() || !newName.trim()) return;
    try {
      const senderIdValue = newSenderIdentityId && newSenderIdentityId !== SENDER_DEFAULT ? newSenderIdentityId : undefined;
      await createMutation.mutateAsync({
        slug: newSlug.trim(),
        name: newName.trim(),
        adapter_id: newAdapterId || undefined,
        sender_identity_id: senderIdValue,
      });
      toast.success("Template type created");
      setNewSlug("");
      setNewName("");
      setNewAdapterId("");
      setNewSenderIdentityId("");
      setSlugTouched(false);
    } catch {
      toast.error("Failed to create template type");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div className="relative w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search template types..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 h-9"
          />
        </div>
        <FormDialog
          trigger={
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              New Template Type
            </Button>
          }
          open={createOpen}
          onOpenChange={setCreateOpen}
          title="Create Template Type"
          description="Define a new template type for this scope."
          submitLabel="Create"
          onSubmit={handleCreateType}
        >
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="tt-name">Name</Label>
              <Input
                id="tt-name"
                placeholder="Welcome Email"
                value={newName}
                onChange={(e) => {
                  const name = e.target.value;
                  setNewName(name);
                  if (!slugTouched) {
                    setNewSlug(generateSlug(name));
                  }
                }}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="tt-slug">Slug</Label>
              <Input
                id="tt-slug"
                placeholder="welcome-email"
                value={newSlug}
                onChange={(e) => {
                  setNewSlug(e.target.value);
                  setSlugTouched(true);
                }}
                className="font-mono"
              />
            </div>
            <AdapterSelect
              adapters={adapters}
              value={newAdapterId}
              onChange={setNewAdapterId}
              senderIdentityId={newSenderIdentityId}
              onSenderIdentityChange={setNewSenderIdentityId}
            />
          </div>
        </FormDialog>
      </div>

      <DataTable
        columns={columns}
        data={filtered}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={FileType}
            title="No template types"
            description="Create your first template type to organize your email templates."
            action={
              <Button variant="outline" size="sm" onClick={() => setCreateOpen(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Create Template Type
              </Button>
            }
          />
        }
      />

      {editTarget && (
        <EditTemplateTypeDialog
          templateType={editTarget}
          adapters={adapters}
          open={!!editTarget}
          onOpenChange={(open) => !open && setEditTarget(null)}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Template Type"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.slug, {
              onSuccess: () => toast.success("Template type deleted"),
              onError: () => toast.error("Failed to delete template type"),
            });
            setDeleteTarget(null);
          }
        }}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}

function EditTemplateTypeDialog({
  templateType,
  adapters,
  open,
  onOpenChange,
}: {
  templateType: TemplateType;
  adapters: { id: string; name: string; adapter_type: string }[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scopedPath = useScopedPath();
  const updateMutation = useUpdateTemplateType(scopedPath, templateType.slug);
  const [adapterId, setAdapterId] = useState(templateType.adapter_id ?? "");
  const [senderIdentityId, setSenderIdentityId] = useState(templateType.sender_identity_id ?? "");

  async function handleSubmit() {
    try {
      const senderIdValue = senderIdentityId && senderIdentityId !== SENDER_DEFAULT ? senderIdentityId : "";
      await updateMutation.mutateAsync({
        adapter_id: adapterId || undefined,
        sender_identity_id: senderIdValue || undefined,
      });
      toast.success("Template type updated");
    } catch {
      toast.error("Failed to update");
    }
  }

  return (
    <FormDialog
      trigger={<span />}
      open={open}
      onOpenChange={onOpenChange}
      title={`Edit — ${templateType.name}`}
      description="Change the adapter and sender assigned to this template type."
      submitLabel="Update"
      onSubmit={handleSubmit}
    >
      <AdapterSelect
        adapters={adapters}
        value={adapterId}
        onChange={setAdapterId}
        senderIdentityId={senderIdentityId}
        onSenderIdentityChange={setSenderIdentityId}
      />
    </FormDialog>
  );
}

function AdapterSelect({
  adapters,
  value,
  onChange,
  senderIdentityId,
  onSenderIdentityChange,
}: {
  adapters: { id: string; name: string; adapter_type: string }[];
  value: string;
  onChange: (value: string) => void;
  senderIdentityId?: string;
  onSenderIdentityChange?: (value: string) => void;
}) {
  const scopedPath = useScopedPath();
  const selectedAdapter = adapters.find((a) => a.id === value);
  const showIdentitySelect = !!selectedAdapter && selectedAdapter.adapter_type === "ses";

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-2">
        <Label>Adapter</Label>
        <Select
          value={value}
          onValueChange={(v) => {
            onChange(v);
            onSenderIdentityChange?.("");
          }}
        >
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Select adapter..." />
          </SelectTrigger>
          <SelectContent>
            {adapters.map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.name} ({a.adapter_type})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {showIdentitySelect && onSenderIdentityChange && (
        <SenderIdentitySelect
          scopedPath={scopedPath}
          adapterId={value}
          value={senderIdentityId ?? ""}
          onChange={onSenderIdentityChange}
        />
      )}
    </div>
  );
}

function SenderIdentitySelect({
  scopedPath,
  adapterId,
  value,
  onChange,
}: {
  scopedPath: string;
  adapterId: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const { data: identities, isLoading } = useIdentityList(scopedPath, adapterId);
  const emailIdentities = (identities ?? []).filter(
    (i) => i.identity_type === "email" && i.status === "verified"
  );

  return (
    <div className="flex flex-col gap-2">
      <Label>Sender Identity</Label>
      <Select value={value} onValueChange={onChange} disabled={isLoading}>
        <SelectTrigger className="w-full font-mono text-sm">
          <SelectValue placeholder={isLoading ? "Loading..." : "Use adapter default"} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={SENDER_DEFAULT}>
            Use adapter default
          </SelectItem>
          {emailIdentities.map((i) => (
            <SelectItem key={i.id} value={i.id} className="font-mono">
              {i.identity}
              {i.display_name ? ` (${i.display_name})` : ""}
            </SelectItem>
          ))}
          {emailIdentities.length === 0 && !isLoading && (
            <SelectItem value={SENDER_NONE} disabled>
              No sender emails configured
            </SelectItem>
          )}
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        Choose which email address to send from. Add senders in the adapter&apos;s identity panel.
      </p>
    </div>
  );
}
